// Package updater checks a signed update endpoint and installs a newer frontend seal.
package updater

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zigslice/xgoef/asset"
	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
	"github.com/zigslice/xgoef/update"
)

// Plugin implements plugin.Plugin.
type Plugin struct {
	current  string
	endpoint string
	pub      ed25519.PublicKey
	host     string
	sibling  string
	state    string
}

// New prepares the updater. An empty endpoint still registers the commands;
// they fail until an endpoint and public key are configured.
func New(current, endpoint, pub, base, hostPath string) (*Plugin, error) {
	sibling := ""
	state := ""
	if base != "" {
		sibling = filepath.Join(base, "xgoef.assets")
		state = filepath.Join(base, ".xgoef", "updater-version")
		if b, err := os.ReadFile(state); err == nil {
			remembered := strings.TrimSpace(string(b))
			if newer, err := update.Newer(current, remembered); err == nil && newer {
				current = remembered
			} else if current == "" && remembered != "" {
				current = remembered
			}
		}
	}
	p := &Plugin{
		current: current,
		host:    hostPath,
		sibling: sibling,
		state:   state,
	}
	if endpoint == "" {
		return p, nil
	}
	resolved, err := update.Resolve(endpoint, base)
	if err != nil {
		return nil, err
	}
	if pub == "" {
		return nil, fmt.Errorf("updater: asset public key is not configured")
	}
	key, err := asset.ParsePublicKey(pub)
	if err != nil {
		return nil, err
	}
	p.endpoint = resolved
	p.pub = key
	return p, nil
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "updater" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{"updater:check", "updater:install"}
}

// Setup registers updater:check and updater:install.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("updater:check", func(ipc.Context, []byte) (any, error) {
		offer, _, ok, err := p.look()
		if err != nil || !ok {
			return nil, err
		}
		return offer, nil
	})
	app.Register("updater:install", func(ipc.Context, []byte) (any, error) {
		offer, sealed, ok, err := p.look()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("updater: already current")
		}
		if err := update.Install(p.host, p.sibling, sealed, p.pub); err != nil {
			return nil, err
		}
		if p.state != "" {
			if err := os.MkdirAll(filepath.Dir(p.state), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(p.state, []byte(offer.Version+"\n"), 0o644); err != nil {
				return nil, err
			}
		}
		p.current = offer.Version
		return map[string]any{"version": offer.Version, "restart": true}, nil
	})
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

func (p *Plugin) look() (update.Offer, []byte, bool, error) {
	if p.endpoint == "" {
		return update.Offer{}, nil, false, fmt.Errorf("updater: endpoint is not configured")
	}
	if len(p.pub) == 0 {
		return update.Offer{}, nil, false, fmt.Errorf("updater: asset public key is not configured")
	}
	m, sealed, err := update.Fetch(p.endpoint)
	if err != nil {
		return update.Offer{}, nil, false, err
	}
	offer, ok, err := update.Check(p.current, m, sealed, p.pub)
	if err != nil || !ok {
		return update.Offer{}, nil, false, err
	}
	return offer, sealed, true, nil
}
