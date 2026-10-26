// Package cli implements the xgoef create / dev / build commands.
package cli

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/zigslice/xgoef/bundle"
	"github.com/zigslice/xgoef/conf"
	"github.com/zigslice/xgoef/deeplink"
)

//go:embed templates
var templates embed.FS

// Run executes one CLI invocation. args is os.Args[1:].
func Run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	case "info":
		fmt.Println("xgoef: XGo + LLGo + CEF client framework")
		fmt.Println("builtins: fs, os, shell, clipboard, dialog")
		fmt.Println("see docs/plugins.md and docs/run.md")
		return 0
	case "create":
		if err := cmdCreate(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "build":
		if err := cmdBuild(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "dev":
		if err := cmdDev(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `xgoef — scaffold CLI for the XGo CEF framework

Usage:
  xgoef info
  xgoef create <name> [--template html|vite|react]
  xgoef dev  [--app dir] [--out dir] [--assets-only]
  xgoef build [--app dir] [--out dir] [--assets-only]

create writes main.xgo, xgoef.conf.json, the frontend, a copy of xgoef.js,
and xgoef.d.ts. html is a static page in web/. vite and react are Vite
projects that build into web/ and open http://127.0.0.1:5173 in dev.
build runs build.beforeBuildCommand before it stages the frontend.
dev runs build.beforeDevCommand and, when devUrl is an http(s) URL, waits
until that address accepts connections. dev rewrites the first window URL
to build.devUrl when that field is set and does not seal, so the loose
frontend stays in use. It then runs the host from the output directory and
recopies the frontend when those files change. The page reloads on the
dev-reload event. Stage also writes web/xgoef.d.ts from conf commands and
the enabled plugins. --assets-only still returns after staging when the
host binary is not in the output directory.
`)
}

func cmdCreate(args []string) error {
	name, template, err := parseCreateArgs(args)
	if err != nil {
		return err
	}
	if !validName(name) {
		return fmt.Errorf("create: name must match [A-Za-z][A-Za-z0-9_-]*")
	}
	if !knownTemplate(template) {
		return fmt.Errorf("create: unknown template %q (%s)", template, templateList)
	}
	if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("create: %s already exists", name)
	}
	js, err := findJS(".")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(name, "api", "js"), 0o755); err != nil {
		return err
	}
	if err := copyTemplate("templates/main.xgo", filepath.Join(name, "main.xgo"), name); err != nil {
		return err
	}
	if err := copyTemplateTree("templates/"+template, name, name); err != nil {
		return err
	}
	if err := copyFile(js, filepath.Join(name, "api", "js", "xgoef.js")); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(name, "web"), 0o755); err != nil {
		return err
	}
	if err := copyFile(js, filepath.Join(name, "web", "xgoef.js")); err != nil {
		return err
	}
	cfg, err := conf.Load(filepath.Join(name, "xgoef.conf.json"))
	if err != nil {
		return err
	}
	if err := emitBindings(cfg, name); err != nil {
		return err
	}
	fmt.Printf("created %s (%s)\n", name, template)
	if template == "html" {
		fmt.Printf("next: xgoef build --app %s --out %s/dist\n", name, name)
		return nil
	}
	fmt.Printf("next: cd %s && npm install\n", name)
	fmt.Printf("next: xgoef dev --app %s\n", name)
	return nil
}

func parseCreateArgs(args []string) (name, template string, err error) {
	template = "html"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--template":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("create: missing value for --template")
			}
			i++
			template = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				return "", "", fmt.Errorf("create: unknown flag %q", args[i])
			}
			if name != "" {
				return "", "", fmt.Errorf("create: expected <name>")
			}
			name = args[i]
		}
	}
	if name == "" {
		return "", "", fmt.Errorf("create: expected <name>")
	}
	return name, template, nil
}

const templateList = "html, vite, react"

func knownTemplate(name string) bool {
	switch name {
	case "html", "vite", "react":
		return true
	default:
		return false
	}
}

func cmdBuild(args []string) error {
	appDir, outDir, assetsOnly, err := parseStageFlags(args, "dist")
	if err != nil {
		return err
	}
	return buildAll(appDir, outDir, false, assetsOnly)
}

func cmdDev(args []string) error {
	appDir, outDir, assetsOnly, err := parseStageFlags(args, filepath.Join("build", "dev"))
	if err != nil {
		return err
	}
	return buildAll(appDir, outDir, true, assetsOnly)
}

func buildAll(appDir, outDir string, dev, assetsOnly bool) error {
	if !dev {
		if err := runBeforeBuild(appDir); err != nil {
			return err
		}
	}
	if !assetsOnly {
		if _, err := linkHost(appDir, outDir); err != nil {
			return err
		}
	}
	dir, err := stage(appDir, outDir, dev)
	if err != nil {
		return err
	}
	if assetsOnly {
		if !dev {
			if err := maybeBundle(dir); err != nil {
				return err
			}
			fmt.Printf("staged %s\n", dir)
			return nil
		}
		return finishDev(appDir, dir, true)
	}
	cfg, err := conf.Load(filepath.Join(appDir, "xgoef.conf.json"))
	if err != nil {
		return err
	}
	bin := filepath.Join(dir, sanitizeBin(cfg.ProductName))
	if !dev {
		if err := sealStaged(appDir, dir, bin); err != nil {
			return err
		}
		fmt.Printf("sealed %s\n", bin)
		if err := maybeBundle(dir); err != nil {
			return err
		}
		fmt.Printf("built %s\n", bin)
		fmt.Printf("run: cd %s && ./%s\n", dir, sanitizeBin(cfg.ProductName))
		return nil
	}
	fmt.Printf("built %s\n", bin)
	return finishDev(appDir, dir, false)
}

func finishDev(appDir, dir string, assetsOnly bool) error {
	cfg, err := conf.Load(filepath.Join(dir, "xgoef.conf.json"))
	if err != nil {
		return err
	}
	bin := filepath.Join(dir, sanitizeBin(cfg.ProductName))
	st, err := os.Stat(bin)
	if err != nil || st.IsDir() {
		if assetsOnly {
			fmt.Printf("staged %s\n", dir)
			return nil
		}
		if err == nil {
			err = fmt.Errorf("not a file")
		}
		return fmt.Errorf("dev: host binary %s: %w", bin, err)
	}
	return serveDev(appDir, dir, bin)
}

func parseStageFlags(args []string, defaultOut string) (appDir, outDir string, assetsOnly bool, err error) {
	appDir = "."
	outDir = ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--app":
			if i+1 >= len(args) {
				return "", "", false, fmt.Errorf("missing value for --app")
			}
			i++
			appDir = args[i]
		case "--out":
			if i+1 >= len(args) {
				return "", "", false, fmt.Errorf("missing value for --out")
			}
			i++
			outDir = args[i]
		case "--assets-only":
			assetsOnly = true
		default:
			return "", "", false, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if outDir == "" {
		outDir = filepath.Join(appDir, defaultOut)
	}
	return appDir, outDir, assetsOnly, nil
}

// stage copies conf, frontendDist, and xgoef.js into outDir.
// When dev is true and build.devUrl is set, the first window URL becomes that URL.
func stage(appDir, outDir string, dev bool) (string, error) {
	cfgPath := filepath.Join(appDir, "xgoef.conf.json")
	cfg, err := conf.Load(cfgPath)
	if err != nil {
		return "", err
	}
	if dev && cfg.Build.DevURL != "" && len(cfg.App.Windows) > 0 {
		cfg.App.Windows[0].URL = cfg.Build.DevURL
	}
	dist := cfg.Build.FrontendDist
	if dist == "" {
		dist = "web"
	}
	srcWeb := dist
	if !filepath.IsAbs(srcWeb) {
		srcWeb = filepath.Join(appDir, dist)
	}
	st, err := os.Stat(srcWeb)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("frontend dist %s: %w", srcWeb, err)
	}
	js, err := findJS(appDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(outDir, "xgoef.conf.json"), raw, 0o644); err != nil {
		return "", err
	}
	dstWeb := filepath.Join(outDir, "web")
	if err := copyTree(srcWeb, dstWeb); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(outDir, "api", "js"), 0o755); err != nil {
		return "", err
	}
	if err := copyFile(js, filepath.Join(outDir, "api", "js", "xgoef.js")); err != nil {
		return "", err
	}
	// xgoef://app/ is rooted at web/, so the bridge is also copied there.
	if err := copyFile(js, filepath.Join(dstWeb, "xgoef.js")); err != nil {
		return "", err
	}
	if err := rewriteBridgeSrc(filepath.Join(dstWeb, "index.html")); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := emitBindings(cfg, outDir); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(outDir)
	if err != nil {
		return outDir, nil
	}
	if err := writeDesktop(abs, cfg); err != nil {
		return "", err
	}
	return abs, nil
}

func maybeBundle(outDir string) error {
	cfg, err := conf.Load(filepath.Join(outDir, "xgoef.conf.json"))
	if err != nil || strings.TrimSpace(cfg.Version) == "" {
		return err
	}
	schemes, err := cfg.DeepLinkSchemes()
	if err != nil {
		return err
	}
	paths, err := bundle.Write(outDir, bundle.Spec{
		Product:    cfg.ProductName,
		Version:    cfg.Version,
		Identifier: cfg.Identifier,
		Schemes:    schemes,
	})
	if err != nil {
		return err
	}
	fmt.Printf("bundled %s\n", paths.Deb)
	fmt.Printf("bundled %s\n", paths.MSI)
	fmt.Printf("bundled %s\n", paths.DMG)
	fmt.Printf("bundled %s\n", paths.AppImage)
	return nil
}

func writeDesktop(outDir string, cfg conf.File) error {
	schemes, err := cfg.DeepLinkSchemes()
	if err != nil || len(schemes) == 0 {
		return err
	}
	name := cfg.ProductName
	if name == "" {
		name = cfg.Identifier
	}
	body, err := deeplink.File(name, filepath.Join(outDir, sanitizeBin(cfg.ProductName)), schemes)
	if err != nil {
		return err
	}
	file, err := deeplink.FileName(cfg.Identifier)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, file), []byte(body), 0o644)
}

func validName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && (r >= '0' && r <= '9' || r == '_' || r == '-'):
		default:
			return false
		}
	}
	return true
}

func copyTemplate(src, dst, name string) error {
	b, err := templates.ReadFile(src)
	if err != nil {
		return err
	}
	body := strings.ReplaceAll(string(b), "__NAME__", name)
	body = strings.ReplaceAll(body, "__PKG__", strings.ToLower(name))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, []byte(body), 0o644)
}

func copyTemplateTree(root, dest, name string) error {
	return fs.WalkDir(templates, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, root)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		if rel == "gitignore" || strings.HasSuffix(rel, "/gitignore") {
			rel = strings.TrimSuffix(rel, "gitignore") + ".gitignore"
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyTemplate(path, target, name)
	})
}

func findJS(appDir string) (string, error) {
	if p := os.Getenv("XGOEF_JS"); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
		return "", fmt.Errorf("XGOEF_JS %s not found", p)
	}
	dir, err := filepath.Abs(appDir)
	if err != nil {
		return "", err
	}
	for i := 0; i < 6; i++ {
		cand := filepath.Join(dir, "api", "js", "xgoef.js")
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("api/js/xgoef.js not found from %s (set XGOEF_JS)", appDir)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func rewriteBridgeSrc(indexPath string) error {
	b, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}
	body := string(b)
	body = strings.ReplaceAll(body, `src="../../../api/js/xgoef.js"`, `src="xgoef.js"`)
	body = strings.ReplaceAll(body, `src="../api/js/xgoef.js"`, `src="xgoef.js"`)
	if body == string(b) {
		return nil
	}
	return os.WriteFile(indexPath, []byte(body), 0o644)
}
