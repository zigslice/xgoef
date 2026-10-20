package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zigslice/xgoef/deeplink"
	"github.com/zigslice/xgoef/ipc"
)

func (a *App) programArgs() []string {
	if a.argsSet {
		return append([]string(nil), a.args...)
	}
	return os.Args
}

// SetArgs replaces the arguments used for single-instance handoff and deep links.
// Tests use it. Production leaves it unset and reads os.Args.
func (a *App) SetArgs(args []string) *App {
	a.args = append([]string(nil), args...)
	a.argsSet = true
	return a
}

// SetDeepLinkDirs overrides the XDG applications directory and the directory
// that holds mimeapps.list. Tests use it.
func (a *App) SetDeepLinkDirs(applications, configDir string) *App {
	a.deepApps = applications
	a.deepConfig = configDir
	return a
}

func (a *App) openDeepLink(args []string) error {
	schemes, err := a.Config.DeepLinkSchemes()
	if err != nil || len(schemes) == 0 {
		return err
	}
	exec := a.executable()
	if exec == "" {
		return fmt.Errorf("deeplink: executable is unknown")
	}
	if abs, err := filepath.Abs(exec); err == nil {
		exec = abs
	}
	apps, cfg := a.deepLinkDirs()
	if apps == "" || cfg == "" {
		return fmt.Errorf("deeplink: application directory is unknown")
	}
	name := a.Config.ProductName
	if name == "" {
		name = a.Config.Identifier
	}
	if _, err := deeplink.Install(apps, cfg, a.Config.Identifier, name, exec, schemes); err != nil {
		return err
	}
	a.Router.Handle("deeplink:get_current", func(ipc.Context, []byte) (any, error) {
		a.deepMu.Lock()
		urls := append([]string(nil), a.deepURLs...)
		a.deepMu.Unlock()
		if urls == nil {
			urls = []string{}
		}
		return map[string]any{"urls": urls}, nil
	})
	a.deliverDeepLink(args)
	return nil
}

func (a *App) deliverDeepLink(args []string) {
	schemes, err := a.Config.DeepLinkSchemes()
	if err != nil || len(schemes) == 0 {
		return
	}
	urls := deeplink.FromArgs(args, schemes)
	if len(urls) == 0 {
		return
	}
	copied := append([]string(nil), urls...)
	a.deepMu.Lock()
	a.deepURLs = copied
	a.deepMu.Unlock()
	_ = a.Emit("deep-link", map[string]any{"urls": append([]string(nil), copied...)})
}

func (a *App) deepLinkDirs() (string, string) {
	if a.deepApps != "" || a.deepConfig != "" {
		return a.deepApps, a.deepConfig
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" && home != "" {
		data = filepath.Join(home, ".local", "share")
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" && home != "" {
		config = filepath.Join(home, ".config")
	}
	if data == "" || config == "" {
		return "", ""
	}
	return filepath.Join(data, "applications"), config
}
