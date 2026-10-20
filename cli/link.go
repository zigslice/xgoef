package cli

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/zigslice/xgoef/conf"
)

// runCommand executes one toolchain command. Tests replace it.
var runCommand = func(name string, args []string, dir string, extraEnv []string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = mergeEnv(extraEnv)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// lookBin resolves a tool on PATH. Tests replace it.
var lookBin = exec.LookPath

// linkHost builds the CEF shim and the XGo app into outDir.
// The caller stages the frontend afterwards so web/ is not overwritten
// by the CEF runtime copy.
func linkHost(appDir, outDir string) (string, error) {
	module, err := findModule(appDir)
	if err != nil {
		return "", err
	}
	cefRoot, err := resolveCEFRoot(module)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(filepath.Join(cefRoot, "include")); err != nil || !st.IsDir() {
		return "", fmt.Errorf("CEF_ROOT missing or incomplete: %s", cefRoot)
	}
	llgoRoot := os.Getenv("LLGO_ROOT")
	if llgoRoot == "" {
		llgoRoot = "/opt/xgo-toolchain/llgo"
	}
	if st, err := os.Stat(filepath.Join(llgoRoot, "runtime")); err != nil || !st.IsDir() {
		return "", fmt.Errorf("LLGO_ROOT invalid: %s", llgoRoot)
	}
	for _, tool := range []string{"xgo", "llgo", "cmake", "pkg-config"} {
		if _, err := lookBin(tool); err != nil {
			if tool == "xgo" || tool == "llgo" {
				return "", fmt.Errorf("%s not found; run ./scripts/install-xgo-toolchain.sh first", tool)
			}
			return "", fmt.Errorf("%s not found", tool)
		}
	}

	shimSrc := filepath.Join(module, "cef", "shim")
	shimBuild := filepath.Join(module, "build", "shim")
	pcDir := filepath.Join(module, "build", "pkgconfig")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(shimBuild, 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(pcDir, 0o755); err != nil {
		return "", err
	}

	jobs := "-j" + strconv.Itoa(runtime.NumCPU())
	if err := runCommand("cmake", []string{
		"-S", shimSrc, "-B", shimBuild, "-G", "Unix Makefiles",
		"-DCMAKE_BUILD_TYPE=Release", "-DUSE_SANDBOX=OFF",
		"-DCMAKE_C_COMPILER=gcc", "-DCMAKE_CXX_COMPILER=g++",
		"-DCEF_ROOT=" + cefRoot,
	}, module, nil); err != nil {
		return "", fmt.Errorf("cmake configure: %w", err)
	}
	if err := runCommand("cmake", []string{
		"--build", shimBuild, "--target", "xgoef_cef", "xgoef_helper", jobs,
	}, module, nil); err != nil {
		return "", fmt.Errorf("cmake build: %w", err)
	}

	lib, err := findNamed(shimBuild, "libxgoef_cef.a")
	if err != nil {
		return "", fmt.Errorf("libxgoef_cef.a not found under %s", shimBuild)
	}
	helper, err := findNamed(shimBuild, "xgoef-helper")
	if err != nil {
		return "", fmt.Errorf("xgoef-helper not found under %s", shimBuild)
	}
	if st, err := os.Stat(helper); err != nil || st.IsDir() || st.Mode()&0o111 == 0 {
		return "", fmt.Errorf("xgoef-helper not found under %s", shimBuild)
	}
	wrapper := filepath.Join(cefRoot, "build", "libcef_dll_wrapper", "libcef_dll_wrapper.a")
	if st, err := os.Stat(wrapper); err != nil || st.IsDir() {
		found, ferr := findNamed(shimBuild, "libcef_dll_wrapper.a")
		if ferr != nil {
			return "", fmt.Errorf("libcef_dll_wrapper.a not found; build CEF wrapper first")
		}
		wrapper = found
	}
	runtimeDir := filepath.Dir(lib)
	if _, err := os.Stat(filepath.Join(runtimeDir, "libcef.so")); err == nil {
		if err := copyTree(runtimeDir, outDir); err != nil {
			return "", err
		}
	} else {
		if err := copyTree(filepath.Join(cefRoot, "Release"), outDir); err != nil {
			return "", err
		}
		if err := copyTree(filepath.Join(cefRoot, "Resources"), outDir); err != nil {
			return "", err
		}
	}
	if err := copyFile(lib, filepath.Join(outDir, "libxgoef_cef.a")); err != nil {
		return "", err
	}
	if err := copyFile(wrapper, filepath.Join(outDir, "libcef_dll_wrapper.a")); err != nil {
		return "", err
	}
	if err := copyFile(wrapper, filepath.Join(runtimeDir, "libcef_dll_wrapper.a")); err != nil {
		return "", err
	}
	if err := copyFile(helper, filepath.Join(outDir, "xgoef-helper")); err != nil {
		return "", err
	}
	if err := os.Chmod(filepath.Join(outDir, "xgoef-helper"), 0o755); err != nil {
		return "", err
	}
	clearLLGoCache(module)

	pc := fmt.Sprintf(`Name: xgoef-cef
Description: xgoef CEF MessageRouter host shim
Version: 0.0.1
Libs: -L%s -lxgoef_cef -lcef_dll_wrapper -lcef -lX11 -lpthread -ldl -lrt -lstdc++
Cflags: -I%s -I%s
`, outDir, shimSrc, cefRoot)
	if err := os.WriteFile(filepath.Join(pcDir, "xgoef-cef.pc"), []byte(pc), 0o644); err != nil {
		return "", err
	}
	env := linkEnv(pcDir, module, llgoRoot)
	if err := runCommand("pkg-config", []string{"--libs", "xgoef-cef"}, module, env); err != nil {
		return "", fmt.Errorf("pkg-config xgoef-cef: %w", err)
	}

	cfg, err := conf.Load(filepath.Join(appDir, "xgoef.conf.json"))
	if err != nil {
		return "", err
	}
	bin := filepath.Join(outDir, sanitizeBin(cfg.ProductName))
	rel, err := filepath.Rel(module, appDir)
	if err != nil {
		return "", err
	}
	pkg := "./" + filepath.ToSlash(rel)
	if rel == "." {
		pkg = "."
	}
	if err := runCommand("go", []string{"get", "github.com/goplus/lib@v0.6.3"}, module, env); err != nil {
		return "", fmt.Errorf("go get: %w", err)
	}
	if err := runCommand("xgo", []string{"build", "-o", bin, pkg}, module, env); err != nil {
		return "", fmt.Errorf("xgo build: %w", err)
	}
	if st, err := os.Stat(bin); err != nil || st.IsDir() {
		return "", fmt.Errorf("xgo build did not produce %s", bin)
	}
	return bin, nil
}

// clearLLGoCache drops a stale LLGo link of this module's cef package.
// LLGo caches pkg-config Libs; a changed -l list otherwise stays linked.
var clearLLGoCache = func(module string) {
	modPath, err := readModulePath(module)
	if err != nil || modPath == "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	pattern := filepath.Join(home, ".cache", "llgo", "build", "*", filepath.FromSlash(modPath), "cef")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}
	for _, m := range matches {
		_ = os.RemoveAll(m)
	}
}

func readModulePath(module string) (string, error) {
	b, err := os.ReadFile(filepath.Join(module, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return fields[1], nil
		}
		return "", fmt.Errorf("go.mod missing module path")
	}
	return "", fmt.Errorf("go.mod missing module path")
}

func linkEnv(pcDir, module, llgoRoot string) []string {
	xcmd := os.Getenv("XGO_GOCMD")
	if xcmd == "" {
		xcmd = filepath.Join(module, "scripts", "xgo-llgo-gocmd.sh")
	}
	pc := pcDir
	if old := os.Getenv("PKG_CONFIG_PATH"); old != "" {
		pc = pcDir + string(os.PathListSeparator) + old
	}
	lib := os.Getenv("LIBRARY_PATH")
	base := "/usr/lib/gcc/x86_64-linux-gnu/14:/usr/lib/x86_64-linux-gnu"
	if lib != "" {
		base = base + ":" + lib
	}
	path := "/usr/local/bin:/usr/local/go-1.27/bin:/usr/lib/llvm-22/bin:" + os.Getenv("HOME") + "/go/bin:" + os.Getenv("PATH")
	return []string{
		"CGO_ENABLED=0",
		"CGO_CFLAGS=",
		"CGO_LDFLAGS=",
		"CGO_CXXFLAGS=",
		"CGO_CPPFLAGS=",
		"CGO_FFLAGS=",
		"LLGO_ROOT=" + llgoRoot,
		"XGO_GOCMD=" + xcmd,
		"PKG_CONFIG_PATH=" + pc,
		"LIBRARY_PATH=" + base,
		"PATH=" + path,
	}
}

func mergeEnv(extra []string) []string {
	over := map[string]string{}
	for _, e := range extra {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			over[k] = v
		}
	}
	var out []string
	for _, e := range os.Environ() {
		k, _, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		if _, drop := over[k]; drop {
			continue
		}
		out = append(out, e)
	}
	out = append(out, extra...)
	return out
}

func findModule(appDir string) (string, error) {
	dir, err := filepath.Abs(appDir)
	if err != nil {
		return "", err
	}
	for i := 0; i < 8; i++ {
		if st, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !st.IsDir() {
			if st, err := os.Stat(filepath.Join(dir, "cef", "shim")); err == nil && st.IsDir() {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("xgoef module (go.mod + cef/shim) not found from %s", appDir)
}

func resolveCEFRoot(module string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("CEF_ROOT")); v != "" {
		return v, nil
	}
	b, err := os.ReadFile(filepath.Join(module, "CEF_VERSION"))
	if err != nil {
		return "", fmt.Errorf("CEF_ROOT is unset and CEF_VERSION is missing: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) == "" || strings.TrimSpace(lines[1]) == "" {
		return "", fmt.Errorf("CEF_VERSION must contain a version and a platform")
	}
	ver := strings.TrimSpace(lines[0])
	plat := strings.TrimSpace(lines[1])
	return filepath.Join(module, "third_party", "cef", "cef_binary_"+ver+"_"+plat), nil
}

func findNamed(root, name string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != name {
			return nil
		}
		found = path
		return fs.SkipAll
	})
	if err != nil && err != fs.SkipAll {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s not found", name)
	}
	return found, nil
}

func sanitizeBin(product string) string {
	var b strings.Builder
	for _, r := range product {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "xgoef"
	}
	return b.String()
}
