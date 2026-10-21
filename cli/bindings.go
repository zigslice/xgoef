package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zigslice/xgoef/binding"
	"github.com/zigslice/xgoef/conf"
)

func emitBindings(cfg conf.File, outDir string) error {
	if err := writeBindings(cfg, outDir); err != nil {
		return err
	}
	fmt.Printf("bindings %s\n", filepath.Join(outDir, "web", "xgoef.d.ts"))
	return nil
}

func writeBindings(cfg conf.File, outDir string) error {
	schemes, err := cfg.DeepLinkSchemes()
	if err != nil {
		return err
	}
	cmds, err := binding.Assemble(cfg.Commands, cfg.Plugins, len(schemes) > 0)
	if err != nil {
		return err
	}
	text, err := binding.Render(cmds)
	if err != nil {
		return err
	}
	webDir := filepath.Join(outDir, "web")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		return err
	}
	apiDir := filepath.Join(outDir, "api", "js")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		return err
	}
	body := []byte(text)
	if err := os.WriteFile(filepath.Join(webDir, "xgoef.d.ts"), body, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(apiDir, "xgoef.d.ts"), body, 0o644)
}
