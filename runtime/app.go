package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/asset"
	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/conf"
	"github.com/zigslice/xgoef/instance"
	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/menu"
	"github.com/zigslice/xgoef/plugin"
	"github.com/zigslice/xgoef/plugins"
	"github.com/zigslice/xgoef/windowstate"
)

// App is the Tauri-like application handle.
type App struct {
	Config   conf.File
	Host     cef.Host
	Bridge   cef.Bridge
	Router   *ipc.Router
	Events   *ipc.Bus
	Channels *ipc.Hub
	ACL      *acl.Guard
	Plugins  *plugin.Registry

	base      string
	exe       string
	assets    *asset.Root
	pluginErr error

	appMenu    *menu.Menu
	trayMenu   *menu.Menu
	trayTip    string
	trayIcon   string
	winState   *windowstate.Store
	instDir    string
	inst       *instance.Listener
	argsSet    bool
	args       []string
	deepApps   string
	deepConfig string
	deepMu     sync.Mutex
	deepURLs   []string
	invokes    *invokeGate
}

// New builds an App with defaults. Host may be nil (uses cef.Unimplemented).
// Bridge may be nil (uses cef.NewNativeBridge).
func New(cfg conf.File, host cef.Host) *App {
	if host == nil {
		host = cef.Unimplemented{}
	}
	bridge := cef.NewNativeBridge()
	bus := ipc.NewBus()
	bus.SetEmitter(bridge)
	hub := ipc.NewHub()
	hub.SetEmitter(bridge)
	router := ipc.NewRouter()
	router.SetChannels(hub)
	return &App{
		Config:   cfg,
		Host:     host,
		Bridge:   bridge,
		Router:   router,
		Events:   bus,
		Channels: hub,
		ACL:      acl.NewGuard(cfg.Capabilities),
		Plugins:  plugin.NewRegistry(),
		invokes:  newInvokeGate(),
	}
}

// Use registers a plugin (Tauri-like builder hook).
// A duplicate or nil plugin is reported from Run / OpenPlugins.
func (a *App) Use(p plugin.Plugin) *App {
	if a.pluginErr != nil {
		return a
	}
	a.pluginErr = a.Plugins.Add(p)
	return a
}

// SetBase sets the app directory used to expand $APP, relative fs paths, and
// the frontend root (build.frontendDist) for xgoef://app/.
func (a *App) SetBase(dir string) *App {
	a.base = dir
	a.assets = nil
	return a
}

// SetInstanceDir overrides the directory of the single-instance socket.
// Tests use it. Production uses XDG_RUNTIME_DIR, or the temp directory.
func (a *App) SetInstanceDir(dir string) *App {
	a.instDir = dir
	return a
}

// SetExecutable overrides the host binary used to read a sealed frontend.
// Tests use it; production leaves it empty and reads os.Executable.
func (a *App) SetExecutable(path string) *App {
	a.exe = path
	a.assets = nil
	return a
}

// Handle registers an IPC command on the app router.
func (a *App) Handle(name string, h ipc.Handler) *App {
	a.Router.Handle(name, h)
	return a
}

// Register implements plugin.Host.
func (a *App) Register(name string, h ipc.Handler) {
	a.Router.Handle(name, h)
}

// OpenPlugins loads builtin plugins named in the config that were not
// already added with Use. It is safe to call more than once.
func (a *App) OpenPlugins() error {
	if a.pluginErr != nil {
		return a.pluginErr
	}
	base := a.base
	if base == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		base = wd
	}
	resource, err := a.resourceDir(base)
	if err != nil {
		return err
	}
	opt := plugins.Options{
		Base:            base,
		Resource:        resource,
		Allow:           a.Config.Plugin.FS.Allow,
		Deny:            a.Config.Plugin.FS.Deny,
		ShellSchemes:    a.Config.Plugin.Shell.Schemes,
		ShellAllow:      a.Config.Plugin.Shell.Allow,
		ShellDeny:       a.Config.Plugin.Shell.Deny,
		DialogAllow:     a.Config.Plugin.Dialog.Allow,
		DialogDeny:      a.Config.Plugin.Dialog.Deny,
		NotifyAllow:     a.Config.Plugin.Notification.Allow,
		NotifyDeny:      a.Config.Plugin.Notification.Deny,
		AppName:         a.Config.ProductName,
		ClipboardMode:   a.Config.Plugin.Clipboard.Mode,
		Version:         a.Config.Version,
		UpdaterEndpoint: a.Config.Plugin.Updater.Endpoint,
		UpdaterPub:      a.Config.Build.AssetPublicKey,
		Executable:      a.executable(),
		Args:            a.programArgs(),
		Release:         a.closeInstance,
	}
	for _, name := range a.Config.Plugins {
		if a.Plugins.Has(name) {
			continue
		}
		p, err := plugins.Open(name, opt)
		if err != nil {
			return err
		}
		if err := a.Plugins.Add(p); err != nil {
			return err
		}
	}
	return nil
}

// Emit sends an event to all webviews (and host listeners).
func (a *App) Emit(name string, payload any) error {
	return a.Events.Emit(name, payload)
}

// EmitTo sends an event to one window label.
func (a *App) EmitTo(windowLabel, name string, payload any) error {
	return a.Events.EmitTo(windowLabel, name, payload)
}

// CreateWindow opens another browser window with the given label.
func (a *App) CreateWindow(spec cef.WindowSpec) error {
	if a.Bridge == nil {
		return fmt.Errorf("runtime: no bridge")
	}
	if spec.URL != "" {
		url, err := a.NavigationURL(spec.URL)
		if err != nil {
			return err
		}
		spec.URL = url
	}
	a.applySavedSize(&spec)
	return a.Bridge.CreateWindow(spec)
}

// CloseWindow closes the window with the given label.
func (a *App) CloseWindow(label string) error {
	if a.Bridge == nil {
		return fmt.Errorf("runtime: no bridge")
	}
	return a.Bridge.CloseWindow(label)
}

// FocusWindow focuses the window with the given label.
func (a *App) FocusWindow(label string) error {
	if a.Bridge == nil {
		return fmt.Errorf("runtime: no bridge")
	}
	return a.Bridge.FocusWindow(label)
}

// Run executes the browser-process side of the CEF lifecycle.
// Helper processes must call Host.ExecuteProcess at the start of main
// (before Run); when Run is entered, this is already the browser process.
// Browser URL is passed as `--url=` for OnContextInitialized.
func (a *App) Run() error {
	reloadCtx, stopReload := context.WithCancel(context.Background())
	defer stopReload()
	go a.serveDevReload(reloadCtx)
	if err := a.acquireInstance(); err != nil {
		return err
	}
	defer a.closeInstance()
	if err := a.openDeepLink(a.programArgs()); err != nil {
		return err
	}

	navURL, err := a.NavigationURL(a.Config.FirstWindow().URL)
	if err != nil {
		return err
	}
	if err := a.OpenWindowState(); err != nil {
		return err
	}
	if nh, ok := a.Host.(interface{ SetArgs([]string) }); ok {
		nh.SetArgs(a.buildArgs(navURL))
	}

	settings := cef.Settings{
		NoSandbox:     true,
		RootCachePath: a.Config.CEF.CacheDir,
	}
	if a.Config.CEF.NoSandbox != nil {
		settings.NoSandbox = *a.Config.CEF.NoSandbox
	}
	if settings.RootCachePath != "" {
		_ = os.MkdirAll(settings.RootCachePath, 0o755)
	}
	settings.SubprocessPath = resolveSubprocess()

	if err := a.OpenPlugins(); err != nil {
		return err
	}
	if err := a.Plugins.Setup(a); err != nil {
		return fmt.Errorf("plugin setup: %w", err)
	}

	a.wireIPC()

	if err := a.OpenChrome(); err != nil {
		return err
	}

	if err := a.Host.Initialize(settings); err != nil {
		return err
	}

	win := a.Config.FirstWindow()
	url := navURL
	bounds := cef.WindowBounds{Width: win.Width, Height: win.Height}
	if err := a.Host.CreateBrowser(url, bounds); err != nil {
		a.Host.Shutdown()
		return err
	}

	a.Host.RunLoop()
	a.Plugins.Teardown(a)
	a.Host.Shutdown()
	return nil
}

func (a *App) wireIPC() {
	if a.Bridge == nil {
		return
	}
	label := a.Config.FirstWindow().Label
	if label == "" {
		label = "main"
	}
	a.Bridge.SetPendingLabel(label)

	router := a.Router
	guard := a.ACL
	bridge := a.Bridge
	events := a.Events

	gate := a.invokes
	bridge.SetQueryHandler(func(queryID int64, windowLabel, requestJSON string) {
		wl := windowLabel
		if wl == "" {
			wl = label
		}
		dispatchFrontend(guard, router, events, gate, wl, requestJSON, queryID, func(resp string) {
			bridge.Respond(queryID, true, resp)
		})
	})
	bridge.SetCancelHandler(func(queryID int64) {
		gate.cancelQuery(queryID)
	})
}

// handleMessage routes invoke / event envelopes from the frontend.
func handleMessage(guard *acl.Guard, router *ipc.Router, events *ipc.Bus, windowLabel, requestJSON string) string {
	var req struct {
		Type    string          `json:"type"`
		ID      int             `json:"id"`
		Cmd     string          `json:"cmd"`
		Args    json.RawMessage `json:"args"`
		Event   string          `json:"event"`
		Payload json.RawMessage `json:"payload"`
		Channel int64           `json:"channel"`
	}
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return failResponse(0, ipc.Fail(ipc.CodeInvalidMessage, "invalid invoke JSON: %s", err.Error()))
	}
	switch req.Type {
	case "", "invoke":
		return handleInvoke(guard, router, windowLabel, req.ID, req.Cmd, req.Args)
	case "event":
		if events != nil && req.Event != "" {
			var payload any
			if len(req.Payload) > 0 {
				_ = json.Unmarshal(req.Payload, &payload)
			}
			// JS → host: local listeners only (avoid echo loop to all webviews).
			events.NotifyLocal(req.Event, payload)
		}
		return okResponse(req.ID, map[string]any{"ok": true})
	case "channel-close":
		if req.Channel < 1 || router == nil || router.Channels() == nil {
			return failResponse(req.ID, ipc.Fail(ipc.CodeMissingChannel, "ipc: missing channel"))
		}
		router.Channels().Close(windowLabel, req.Channel)
		return okResponse(req.ID, map[string]any{"ok": true})
	default:
		return failResponse(req.ID, ipc.Fail(ipc.CodeUnsupported, "unsupported message type %s", req.Type))
	}
}

func handleInvoke(guard *acl.Guard, router *ipc.Router, windowLabel string, id int, cmd string, argsJSON json.RawMessage) string {
	if cmd == "" {
		return failResponse(id, ipc.Fail(ipc.CodeMissingCmd, "missing cmd"))
	}
	if guard == nil || !guard.Allows(windowLabel, cmd) {
		return failResponse(id, &acl.DenyError{Window: windowLabel, Command: cmd})
	}
	args := []byte(argsJSON)
	if len(args) == 0 {
		args = []byte("{}")
	}
	out, err := router.InvokeFrom(windowLabel, cmd, args)
	if err != nil {
		return failResponse(id, err)
	}
	var payload any
	if err := json.Unmarshal(out, &payload); err != nil {
		return failResponse(id, ipc.Fail(ipc.CodeBadResponse, "marshal response: %s", err.Error()))
	}
	return okResponse(id, payload)
}

type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func okResponse(id int, payload any) string {
	return writeResponse(id, true, payload, nil)
}

func failResponse(id int, err error) string {
	code, message := ipc.Describe(err)
	return writeResponse(id, false, nil, &responseError{Code: code, Message: message})
}

func writeResponse(id int, ok bool, payload any, err *responseError) string {
	msg := map[string]any{
		"type": "response",
		"id":   id,
		"ok":   ok,
	}
	if ok {
		msg["payload"] = payload
	} else {
		if err == nil {
			err = &responseError{Code: ipc.CodeFailed, Message: "invoke failed"}
		}
		msg["error"] = err
	}
	b, mErr := json.Marshal(msg)
	if mErr != nil {
		return `{"type":"response","id":0,"ok":false,"error":{"code":"ipc.encode","message":"response encode failed"}}`
	}
	return string(b)
}

func (a *App) buildArgs(url string) []string {
	win := a.Config.FirstWindow()
	label := win.Label
	if label == "" {
		label = "main"
	}
	args := append([]string{}, os.Args...)
	args = append(args, "--url="+url, "--xgoef-label="+label)
	if w, h := a.launchSize(label, win.Width, win.Height); w > 0 && h > 0 {
		args = append(args, fmt.Sprintf("--xgoef-size=%d,%d", w, h))
	}
	if a.assets != nil && a.assets.Dir() != "" {
		args = append(args, "--xgoef-asset-root="+a.assets.Dir())
	}
	// LLGo (and Go) hosts are poor CEF GPU/renderer subprocesses.
	// Prefer a native helper via SubprocessPath; these flags keep the
	// browser process alive when the helper still cannot use GPU.
	if os.Getenv("XGOEF_ENABLE_GPU") != "1" {
		args = append(args, "--disable-gpu", "--in-process-gpu")
	}
	return args
}

func resolveSubprocess() string {
	if p := os.Getenv("XGOEF_HELPER"); p != "" {
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(filepath.Dir(exe), "xgoef-helper")
	if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
		return candidate
	}
	return ""
}

// NavigationURL is the URL passed to CEF.
// http(s) and about:blank are kept for dev servers. xgoef://app/ is checked
// against the frontend root. file:// and relative paths inside that root are
// rewritten to xgoef://app/. Anything else on disk is refused.
func (a *App) NavigationURL(u string) (string, error) {
	if u == "" {
		u = asset.Scheme + "://" + asset.Host + "/index.html"
	}
	if u == "about:blank" || strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		_, _ = a.frontend()
		return u, nil
	}
	root, err := a.frontend()
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(strings.ToLower(u), asset.Scheme+"://") {
		if _, _, err := root.Resolve(u); err != nil {
			return "", err
		}
		return u, nil
	}
	path := u
	if strings.HasPrefix(path, "file://") {
		path = strings.TrimPrefix(path, "file://")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(a.baseDir(), path)
	}
	return root.FileURL(path)
}

func (a *App) resourceDir(base string) (string, error) {
	if a.Config.Build.AssetPublicKey != "" {
		root, err := a.frontend()
		if err != nil {
			return "", err
		}
		return root.Dir(), nil
	}
	resource := a.Config.Build.FrontendDist
	if resource == "" {
		resource = "web"
	}
	if !filepath.IsAbs(resource) {
		resource = filepath.Join(base, resource)
	}
	return resource, nil
}

func (a *App) frontend() (*asset.Root, error) {
	if a.assets != nil {
		return a.assets, nil
	}
	if a.Config.Build.AssetPublicKey != "" {
		dir, err := a.materializeSealed()
		if err != nil {
			return nil, err
		}
		root, err := asset.Open(dir)
		if err != nil {
			return nil, err
		}
		a.assets = root
		return root, nil
	}
	dist := a.Config.Build.FrontendDist
	if dist == "" {
		dist = "web"
	}
	if !filepath.IsAbs(dist) {
		dist = filepath.Join(a.baseDir(), dist)
	}
	root, err := asset.Open(dist)
	if err != nil {
		return nil, err
	}
	a.assets = root
	return root, nil
}

func (a *App) materializeSealed() (string, error) {
	pub, err := asset.ParsePublicKey(a.Config.Build.AssetPublicKey)
	if err != nil {
		return "", err
	}
	exe := a.exe
	if exe == "" {
		exe, err = os.Executable()
		if err != nil {
			return "", err
		}
	}
	payload, err := asset.ReadSealedPayload(exe, pub)
	if err == asset.ErrNoSeal {
		sibling := filepath.Join(a.baseDir(), "xgoef.assets")
		payload, err = asset.ReadSealedPayload(sibling, pub)
		if os.IsNotExist(err) {
			return "", fmt.Errorf("asset: sealed frontend missing")
		}
	}
	if err != nil {
		return "", err
	}
	dest := filepath.Join(a.baseDir(), ".xgoef", "unpacked")
	if err := asset.ExtractZip(payload, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func (a *App) executable() string {
	if a.exe != "" {
		return a.exe
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

func (a *App) baseDir() string {
	if a.base != "" {
		return a.base
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// ExitError means this process should exit with Code.
// CEF helpers use their helper code. A second instance uses 0 after the
// first process has accepted the handoff.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("cef helper exit code %d", e.Code)
}
