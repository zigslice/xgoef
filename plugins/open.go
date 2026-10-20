// Package plugins loads the builtin plugin set named in xgoef.conf.json.
package plugins

import (
	"fmt"

	"github.com/zigslice/xgoef/plugin"
	"github.com/zigslice/xgoef/plugins/fs"
	osinfo "github.com/zigslice/xgoef/plugins/os"
)

// Options configures builtin plugins. Base is the app directory ($APP).
// Resource is the frontend dist directory ($RESOURCE). Allow and Deny apply
// only to the fs plugin.
type Options struct {
	Base     string
	Resource string
	Allow    []string
	Deny     []string
}

// Open returns a builtin plugin by name ("fs", "os").
func Open(name string, opt Options) (plugin.Plugin, error) {
	switch name {
	case "fs":
		return fs.New(opt.Base, opt.Resource, opt.Allow, opt.Deny)
	case "os":
		return osinfo.New(), nil
	default:
		return nil, fmt.Errorf("plugins: unknown builtin %q", name)
	}
}
