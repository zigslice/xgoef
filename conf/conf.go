package conf

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/autostart"
	"github.com/zigslice/xgoef/binding"
	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/deeplink"
	"github.com/zigslice/xgoef/eval"
	"github.com/zigslice/xgoef/log"
	"github.com/zigslice/xgoef/remote"
)

// File is the on-disk xgoef.conf.json shape (Tauri-like).
type File struct {
	ProductName  string            `json:"productName"`
	Version      string            `json:"version"`
	Identifier   string            `json:"identifier"`
	Build        Build             `json:"build"`
	App          AppSection        `json:"app"`
	CEF          CEFSection        `json:"cef"`
	Plugins      []string          `json:"plugins"`
	Plugin       PluginSection     `json:"plugin"`
	Commands     []binding.Command `json:"commands,omitempty"`
	Capabilities []acl.Capability  `json:"capabilities"`
}

// PluginSection holds per-plugin settings. Scope roots use $APP, $RESOURCE,
// and $HOME (see plugins/fs).
type PluginSection struct {
	FS             FSConfig           `json:"fs"`
	Shell          ShellConfig        `json:"shell"`
	Dialog         DialogConfig       `json:"dialog"`
	Clipboard      ClipboardConfig    `json:"clipboard"`
	Updater        UpdaterConfig      `json:"updater"`
	Notification   NotificationConfig `json:"notification"`
	GlobalShortcut ShortcutConfig     `json:"global-shortcut"`
	WebView        WebViewConfig      `json:"webview"`
	Eval           EvalConfig         `json:"eval"`
	Autostart      AutostartConfig    `json:"autostart"`
	Log            LogConfig          `json:"log"`
}

// LogConfig is the minimum page level written by log:write.
// Empty means info. Crash records are not filtered by this level.
type LogConfig struct {
	Level string `json:"level"`
}

// AutostartConfig is the launch line written into the user autostart entry.
// The page cannot choose the executable or these arguments.
type AutostartConfig struct {
	Args []string `json:"args"`
}

// EvalConfig is the set of named scripts the page may ask to run.
// The page sends a name. It cannot send the source.
type EvalConfig struct {
	Scripts map[string]string `json:"scripts"`
}

// WebViewConfig gates developer tools. Omitted devtools is false, so a page
// cannot open them with only the capability. Print does not read this flag.
type WebViewConfig struct {
	DevTools bool `json:"devtools"`
}

// UpdaterConfig points at a directory or https URL that serves update.json.
type UpdaterConfig struct {
	Endpoint string `json:"endpoint"`
}

// FSConfig is the scope for the fs builtin.
type FSConfig struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

// ShellConfig limits shell:open. Empty schemes means http and https.
// Empty allow rejects local paths.
type ShellConfig struct {
	Schemes []string `json:"schemes"`
	Allow   []string `json:"allow"`
	Deny    []string `json:"deny"`
}

// DialogConfig is the scope applied to the path a picker returns.
type DialogConfig struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

// NotificationConfig is the scope for an optional notification icon.
// An empty allow list still permits a title and body.
type NotificationConfig struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

// ShortcutConfig lists accelerators the page may register.
// An empty allow list rejects every shortcut.
type ShortcutConfig struct {
	Allow []string `json:"allow"`
}

// ClipboardConfig selects the text store. Mode is "memory" (default) or "system".
type ClipboardConfig struct {
	Mode string `json:"mode"`
}

// Build holds frontend paths and the shell commands that produce them.
// AssetPublicKey is the Ed25519 public key that must verify the sealed
// frontend. Empty means the loose frontendDist directory is used.
// BeforeDevCommand runs from xgoef dev before the host starts.
// BeforeBuildCommand runs from xgoef build before the frontend is staged.
// Both are one-line shell commands. Empty means the step is skipped.
type Build struct {
	FrontendDist       string `json:"frontendDist"`
	DevURL             string `json:"devUrl"`
	AssetPublicKey     string `json:"assetPublicKey,omitempty"`
	BeforeDevCommand   string `json:"beforeDevCommand,omitempty"`
	BeforeBuildCommand string `json:"beforeBuildCommand,omitempty"`
}

// CSPMaxBytes is the longest Content-Security-Policy accepted from config.
const CSPMaxBytes = 2048

// DefaultCSP is the policy sent on xgoef://app/ responses when app.csp is
// omitted. Inline scripts and styles stay allowed because the html template
// and examples/hello put script in the page. A tighter policy means moving
// those scripts into files.
const DefaultCSP = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'"

// AppSection holds window definitions plus the app menu and tray menu.
// WindowState nil means remembered geometry is on.
// WithGlobalXgoef nil means the renderer installs window.__XGOEF__
// before page scripts. False passes --xgoef-no-bridge. This is the
// withGlobalTauri equivalent; the default here is on.
// CSP nil uses DefaultCSP. An empty string disables the asset-response header.
// Remote lists extra http(s) origins. build.devUrl's origin is added at runtime.
type AppSection struct {
	Windows         []Window        `json:"windows"`
	Menu            []MenuItem      `json:"menu,omitempty"`
	Tray            *TrayConfig     `json:"tray,omitempty"`
	WindowState     *bool           `json:"windowState,omitempty"`
	SingleInstance  *bool           `json:"singleInstance,omitempty"`
	DeepLink        *DeepLinkConfig `json:"deepLink,omitempty"`
	WithGlobalXgoef *bool           `json:"withGlobalXgoef,omitempty"`
	CSP             *string         `json:"csp,omitempty"`
	Remote          RemoteConfig    `json:"remote,omitempty"`
}

// RemoteConfig is the http(s) origin allowlist.
// Each entry is an origin. A path is not a scope. The page cannot set this.
type RemoteConfig struct {
	URLs []string `json:"urls,omitempty"`
}

// ContentSecurityPolicy is the header value for xgoef://app/ responses.
// A nil field uses DefaultCSP. An empty string disables the header.
// The page cannot choose this value. devUrl responses are not rewritten.
func (a AppSection) ContentSecurityPolicy() (policy string, enabled bool) {
	if a.CSP == nil {
		return DefaultCSP, true
	}
	if *a.CSP == "" {
		return "", false
	}
	return *a.CSP, true
}

// DeepLinkConfig lists custom URL schemes this application accepts.
type DeepLinkConfig struct {
	Schemes []string `json:"schemes,omitempty"`
}

// DeepLinkSchemes returns the custom schemes to register.
// An empty list means deep link is off. Schemes require an identifier.
func (f File) DeepLinkSchemes() ([]string, error) {
	if f.App.DeepLink == nil || len(f.App.DeepLink.Schemes) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(f.Identifier) == "" {
		return nil, fmt.Errorf("conf: deepLink requires identifier")
	}
	return deeplink.Normalize(f.App.DeepLink.Schemes)
}

// WantSingleInstance reports whether a second launch should hand off to the
// first. An omitted flag is on when identifier is set. An explicit true with
// an empty identifier is an error.
func (f File) WantSingleInstance() (bool, error) {
	if f.App.SingleInstance != nil && !*f.App.SingleInstance {
		return false, nil
	}
	explicit := f.App.SingleInstance != nil && *f.App.SingleInstance
	if strings.TrimSpace(f.Identifier) == "" {
		if explicit {
			return false, fmt.Errorf("conf: singleInstance requires identifier")
		}
		return false, nil
	}
	return true, nil
}

// RememberWindows reports whether places are saved under .xgoef/window-state.json.
func (a AppSection) RememberWindows() bool {
	return a.WindowState == nil || *a.WindowState
}

// InjectBridge reports whether OnContextCreated should eval xgoef.js.
func (a AppSection) InjectBridge() bool {
	return a.WithGlobalXgoef == nil || *a.WithGlobalXgoef
}

// MenuItem is one menu or tray row. ID is required except on separators.
// Enabled defaults to true when omitted.
type MenuItem struct {
	ID        string     `json:"id,omitempty"`
	Text      string     `json:"text,omitempty"`
	Command   string     `json:"command,omitempty"`
	Enabled   *bool      `json:"enabled,omitempty"`
	Separator bool       `json:"separator,omitempty"`
	Items     []MenuItem `json:"items,omitempty"`
}

// TrayConfig is the status item. Menu is shown from the icon.
type TrayConfig struct {
	Tooltip string     `json:"tooltip,omitempty"`
	Icon    string     `json:"icon,omitempty"`
	Menu    []MenuItem `json:"menu,omitempty"`
}

// Window is one browser window.
// Decorations nil keeps the frame. Theme empty follows the system.
// Transparent nil keeps an opaque page background.
type Window struct {
	Label       string   `json:"label"`
	Title       string   `json:"title"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	URL         string   `json:"url"`
	InitScripts []string `json:"initScripts,omitempty"`
	Decorations *bool    `json:"decorations,omitempty"`
	Theme       string   `json:"theme,omitempty"`
	Transparent *bool    `json:"transparent,omitempty"`
	// Resizable, Minimizable, Maximizable, and Closable nil mean allowed.
	Resizable   *bool `json:"resizable,omitempty"`
	Minimizable *bool `json:"minimizable,omitempty"`
	Maximizable *bool `json:"maximizable,omitempty"`
	Closable    *bool `json:"closable,omitempty"`
	// MinWidth and MinHeight are both 0 when unset. The same rule applies to the maximum.
	MinWidth  int `json:"minWidth,omitempty"`
	MinHeight int `json:"minHeight,omitempty"`
	MaxWidth  int `json:"maxWidth,omitempty"`
	MaxHeight int `json:"maxHeight,omitempty"`
}

// Limits maps this window onto the text passed to the CEF shim.
// Omitted flags stay allowed. A size bound is set only when both sides are present.
func (w Window) Limits() (cef.WindowLimits, error) {
	lim := cef.WindowLimits{
		Resizable:   flagOr(w.Resizable, true),
		Minimizable: flagOr(w.Minimizable, true),
		Maximizable: flagOr(w.Maximizable, true),
		Closable:    flagOr(w.Closable, true),
		MinWidth:    w.MinWidth,
		MinHeight:   w.MinHeight,
		MaxWidth:    w.MaxWidth,
		MaxHeight:   w.MaxHeight,
	}
	if err := lim.Validate(); err != nil {
		return cef.WindowLimits{}, fmt.Errorf("conf: window %s %w", w.Label, err)
	}
	return lim, nil
}

func flagOr(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

// Chrome maps this window onto the text passed to the CEF shim.
func (w Window) Chrome() cef.WindowChrome {
	ch := cef.WindowChrome{Theme: w.Theme}
	if w.Decorations != nil && !*w.Decorations {
		ch.Frameless = true
	}
	if w.Transparent != nil && *w.Transparent {
		ch.Transparent = true
	}
	return ch
}

// CEFSection points at the binary distribution.
type CEFSection struct {
	RootEnv   string `json:"rootEnv"`
	NoSandbox *bool  `json:"noSandbox"`
	CacheDir  string `json:"cacheDir"`
}

// rawFile accepts both the P0 capabilities array and the legacy map form.
type rawFile struct {
	ProductName  string            `json:"productName"`
	Version      string            `json:"version"`
	Identifier   string            `json:"identifier"`
	Build        Build             `json:"build"`
	App          AppSection        `json:"app"`
	CEF          CEFSection        `json:"cef"`
	Plugins      []string          `json:"plugins"`
	Plugin       PluginSection     `json:"plugin"`
	Commands     []binding.Command `json:"commands"`
	Capabilities json.RawMessage   `json:"capabilities"`
}

// Load reads and decodes a config file.
func Load(path string) (File, error) {
	var f File
	data, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	var raw rawFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return f, fmt.Errorf("conf: %w", err)
	}
	f.ProductName = raw.ProductName
	f.Version = raw.Version
	f.Identifier = raw.Identifier
	f.Build = raw.Build
	f.App = raw.App
	f.CEF = raw.CEF
	f.Plugins = raw.Plugins
	f.Plugin = raw.Plugin
	seenCmd := map[string]bool{}
	f.Commands = make([]binding.Command, len(raw.Commands))
	for i, cmd := range raw.Commands {
		norm, err := binding.Normalize(cmd)
		if err != nil {
			return f, err
		}
		if seenCmd[norm.Name] {
			return f, fmt.Errorf("binding: duplicate command %q", norm.Name)
		}
		seenCmd[norm.Name] = true
		f.Commands[i] = norm
	}

	caps, err := parseCapabilities(raw.Capabilities)
	if err != nil {
		return f, err
	}
	f.Capabilities = caps

	if f.ProductName == "" {
		return f, fmt.Errorf("conf: productName is required")
	}
	if f.CEF.RootEnv == "" {
		f.CEF.RootEnv = "CEF_ROOT"
	}
	if f.Build.FrontendDist == "" {
		f.Build.FrontendDist = "web"
	}
	for i := range f.App.Windows {
		if f.App.Windows[i].Label == "" {
			f.App.Windows[i].Label = "main"
		}
		if err := normalizeWindowTheme(&f.App.Windows[i]); err != nil {
			return f, err
		}
		if _, err := f.App.Windows[i].Limits(); err != nil {
			return f, err
		}
		if err := eval.ValidateSources(f.App.Windows[i].InitScripts, eval.MaxInitScripts); err != nil {
			return f, fmt.Errorf("conf: window %s: %w", f.App.Windows[i].Label, err)
		}
	}
	if err := eval.ValidateNamed(f.Plugin.Eval.Scripts); err != nil {
		return f, fmt.Errorf("conf: %w", err)
	}
	if err := autostart.ValidateArgs(f.Plugin.Autostart.Args); err != nil {
		return f, fmt.Errorf("conf: %w", err)
	}
	if err := log.ValidateLevel(f.Plugin.Log.Level); err != nil {
		return f, fmt.Errorf("conf: %w", err)
	}
	if err := validShellCommand(f.Build.BeforeDevCommand); err != nil {
		return f, fmt.Errorf("conf: beforeDevCommand: %w", err)
	}
	if err := validShellCommand(f.Build.BeforeBuildCommand); err != nil {
		return f, fmt.Errorf("conf: beforeBuildCommand: %w", err)
	}
	if err := validCSP(f.App.CSP); err != nil {
		return f, fmt.Errorf("conf: %w", err)
	}
	urls, err := remote.Normalize(f.App.Remote.URLs)
	if err != nil {
		return f, fmt.Errorf("conf: %w", err)
	}
	f.App.Remote.URLs = urls
	return f, nil
}

func validCSP(policy *string) error {
	if policy == nil || *policy == "" {
		return nil
	}
	if len(*policy) > CSPMaxBytes {
		return fmt.Errorf("csp is too long")
	}
	for _, r := range *policy {
		if r < 0x20 {
			return fmt.Errorf("csp is invalid")
		}
	}
	return nil
}

func validShellCommand(command string) error {
	if command == "" {
		return nil
	}
	if len(command) > 1024 {
		return fmt.Errorf("invalid")
	}
	for _, r := range command {
		if r < 0x20 {
			return fmt.Errorf("invalid")
		}
	}
	return nil
}

func normalizeWindowTheme(w *Window) error {
	raw := w.Theme
	theme := strings.ToLower(strings.TrimSpace(raw))
	switch theme {
	case "", "system", "light", "dark":
		w.Theme = theme
		return nil
	default:
		return fmt.Errorf("conf: window %s theme %q", w.Label, raw)
	}
}

func parseCapabilities(raw json.RawMessage) ([]acl.Capability, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	// Prefer Tauri-like array form.
	var arr []acl.Capability
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	// Legacy scaffold: { "main": { "commands": ["greet"] } }
	var legacy map[string]struct {
		Commands []string `json:"commands"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, fmt.Errorf("conf: capabilities: %w", err)
	}
	out := make([]acl.Capability, 0, len(legacy))
	for id, v := range legacy {
		out = append(out, acl.Capability{
			Identifier:  id,
			Windows:     []string{id},
			Permissions: append([]string(nil), v.Commands...),
		})
	}
	return out, nil
}

// FirstWindow returns the first window or a default.
func (f File) FirstWindow() Window {
	if len(f.App.Windows) == 0 {
		return Window{Label: "main", Title: f.ProductName, Width: 960, Height: 640, URL: "about:blank"}
	}
	w := f.App.Windows[0]
	if w.Label == "" {
		w.Label = "main"
	}
	return w
}
