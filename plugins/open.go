// Package plugins loads the builtin plugin set named in xgoef.conf.json.
package plugins

import (
	"fmt"

	"github.com/zigslice/xgoef/plugin"
	"github.com/zigslice/xgoef/plugins/clipboard"
	"github.com/zigslice/xgoef/plugins/dialog"
	"github.com/zigslice/xgoef/plugins/fs"
	"github.com/zigslice/xgoef/plugins/globalshortcut"
	"github.com/zigslice/xgoef/plugins/monitor"
	"github.com/zigslice/xgoef/plugins/notification"
	osinfo "github.com/zigslice/xgoef/plugins/os"
	"github.com/zigslice/xgoef/plugins/process"
	"github.com/zigslice/xgoef/plugins/shell"
	"github.com/zigslice/xgoef/plugins/updater"
)

// Options configures builtin plugins. Base is the app directory ($APP).
// Resource is the frontend dist directory ($RESOURCE). Allow and Deny apply
// to the fs plugin. Shell and dialog have their own scope lists.
type Options struct {
	Base            string
	Resource        string
	Allow           []string
	Deny            []string
	ShellSchemes    []string
	ShellAllow      []string
	ShellDeny       []string
	DialogAllow     []string
	DialogDeny      []string
	NotifyAllow     []string
	NotifyDeny      []string
	ShortcutAllow   []string
	AppName         string
	ClipboardMode   string
	Version         string
	UpdaterEndpoint string
	UpdaterPub      string
	Executable      string
	Args            []string
	Release         func()
	Emit            func(name string, payload any) error
	Displays        func(window string) (string, error)
	MoveWindow      func(window string, x, y int) error
}

// Open returns a builtin plugin by name ("fs", "os", "shell", "clipboard", "dialog", "updater", "process", "notification", "global-shortcut", "monitor").
func Open(name string, opt Options) (plugin.Plugin, error) {
	switch name {
	case "fs":
		return fs.New(opt.Base, opt.Resource, opt.Allow, opt.Deny)
	case "os":
		return osinfo.New(), nil
	case "shell":
		return shell.New(opt.Base, opt.Resource, opt.ShellSchemes, opt.ShellAllow, opt.ShellDeny)
	case "clipboard":
		switch opt.ClipboardMode {
		case "", "memory":
			return clipboard.New(nil), nil
		case "system":
			return clipboard.NewSystem()
		default:
			return nil, fmt.Errorf("plugins: clipboard mode %q", opt.ClipboardMode)
		}
	case "dialog":
		return dialog.New(opt.Base, opt.Resource, opt.DialogAllow, opt.DialogDeny, nil)
	case "updater":
		return updater.New(opt.Version, opt.UpdaterEndpoint, opt.UpdaterPub, opt.Base, opt.Executable)
	case "process":
		return process.New(process.Spec{
			Executable: opt.Executable,
			Args:       opt.Args,
			Release:    opt.Release,
		}), nil
	case "notification":
		return notification.New(opt.Base, opt.Resource, opt.AppName, opt.NotifyAllow, opt.NotifyDeny, nil)
	case "global-shortcut":
		return globalshortcut.New(opt.ShortcutAllow, opt.Emit)
	case "monitor":
		return monitor.New(opt.Displays, opt.MoveWindow), nil
	default:
		return nil, fmt.Errorf("plugins: unknown builtin %q", name)
	}
}
