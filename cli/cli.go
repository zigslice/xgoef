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

	"github.com/zigslice/xgoef/conf"
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
		fmt.Println("builtins: fs (scoped), os")
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
  xgoef create <name>
  xgoef dev  [--app dir] [--out dir]
  xgoef build [--app dir] [--out dir]

create writes main.xgo, xgoef.conf.json, web/, and a copy of api/js/xgoef.js.
build / dev stage a copyable directory (conf, frontend, JS bridge).
dev rewrites the first window URL to build.devUrl when that field is set.
Linking the CEF host still uses scripts/build-xgoef-hello.sh.
`)
}

func cmdCreate(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("create: expected <name>")
	}
	name := args[0]
	if !validName(name) {
		return fmt.Errorf("create: name must match [A-Za-z][A-Za-z0-9_-]*")
	}
	if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("create: %s already exists", name)
	}
	js, err := findJS(".")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(name, "web"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(name, "api", "js"), 0o755); err != nil {
		return err
	}
	if err := copyTemplate("templates/main.xgo", filepath.Join(name, "main.xgo"), name); err != nil {
		return err
	}
	if err := copyTemplate("templates/xgoef.conf.json", filepath.Join(name, "xgoef.conf.json"), name); err != nil {
		return err
	}
	if err := copyTemplate("templates/web/index.html", filepath.Join(name, "web", "index.html"), name); err != nil {
		return err
	}
	if err := copyTemplate("templates/web/note.txt", filepath.Join(name, "web", "note.txt"), name); err != nil {
		return err
	}
	if err := copyFile(js, filepath.Join(name, "api", "js", "xgoef.js")); err != nil {
		return err
	}
	fmt.Printf("created %s\n", name)
	fmt.Printf("next: cd %s && xgoef build --app . --out dist\n", name)
	return nil
}

func cmdBuild(args []string) error {
	appDir, outDir, err := parseStageFlags(args, "dist")
	if err != nil {
		return err
	}
	dir, err := stage(appDir, outDir, false)
	if err != nil {
		return err
	}
	fmt.Printf("staged %s\n", dir)
	fmt.Println("link the host with scripts/build-xgoef-hello.sh when building examples/hello")
	return nil
}

func cmdDev(args []string) error {
	appDir, outDir, err := parseStageFlags(args, filepath.Join("build", "dev"))
	if err != nil {
		return err
	}
	dir, err := stage(appDir, outDir, true)
	if err != nil {
		return err
	}
	fmt.Printf("dev stage %s\n", dir)
	return nil
}

func parseStageFlags(args []string, defaultOut string) (appDir, outDir string, err error) {
	appDir = "."
	outDir = ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--app":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("missing value for --app")
			}
			i++
			appDir = args[i]
		case "--out":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("missing value for --out")
			}
			i++
			outDir = args[i]
		default:
			return "", "", fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if outDir == "" {
		outDir = filepath.Join(appDir, defaultOut)
	}
	return appDir, outDir, nil
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
	if err := rewriteBridgeSrc(filepath.Join(dstWeb, "index.html")); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	abs, err := filepath.Abs(outDir)
	if err != nil {
		return outDir, nil
	}
	return abs, nil
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
	return os.WriteFile(dst, []byte(body), 0o644)
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
	body := strings.ReplaceAll(string(b), `src="../../../api/js/xgoef.js"`, `src="../api/js/xgoef.js"`)
	if body == string(b) {
		return nil
	}
	return os.WriteFile(indexPath, []byte(body), 0o644)
}
