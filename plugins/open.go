// Package plugins loads the builtin plugin set named in xgoef.conf.json.
package plugins

import (
	"fmt"

	"github.com/zigslice/xgoef/plugin"
	"github.com/zigslice/xgoef/plugins/clipboard"
	"github.com/zigslice/xgoef/plugins/dialog"
	"github.com/zigslice/xgoef/plugins/fs"
	osinfo "github.com/zigslice/xgoef/plugins/os"
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
	ClipboardMode   string
	Version         string
	UpdaterEndpoint string
	UpdaterPub      string
	Executable      string
}

// Open returns a builtin plugin by name ("fs", "os", "shell", "clipboard", "dialog", "updater").
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
	default:
		return nil, fmt.Errorf("plugins: unknown builtin %q", name)
	}
}
