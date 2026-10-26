package binding

import (
	"strings"
	"testing"
)

func TestRenderAppAndOS(t *testing.T) {
	cmds, err := Assemble([]Command{{
		Name:   "greet",
		Doc:    "hello",
		Args:   []Arg{{Name: "name", Type: " string ", Optional: true}},
		Result: "string",
	}}, []string{"os"}, false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Render(cmds)
	if err != nil {
		t.Fatal(err)
	}
	if got != renderWant {
		t.Fatalf("got:\n%s", got)
	}
}

func TestAssemblePluginsDeepLinkAndRejects(t *testing.T) {
	cmds, err := Assemble(nil, []string{"fs", "shell", "dialog", "updater", "process", "notification", "global-shortcut", "monitor", "webview", "eval", "autostart", "log", "taskbar"}, true)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Render(cmds)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{
		`"fs:read_text": { args: { path: string }; result: string }`,
		`"fs:list": { args: { path: string }; result: { name: string; isDir: boolean }[] }`,
		`"fs:write_text": { args: { path: string; contents: string }; result: null }`,
		"readText(path: string): XgoefInvokeResult<string>",
		"writeText(path: string, contents: string): XgoefInvokeResult<null>",
		"open(target: string): XgoefInvokeResult<string>",
		"open(opts?: { title?: string; defaultPath?: string; directory?: boolean }): XgoefInvokeResult<string | null>",
		"save(opts?: { title?: string; defaultPath?: string; directory?: boolean }): XgoefInvokeResult<string | null>",
		"check(): XgoefInvokeResult<{ version: string; notes?: string; current: string } | null>",
		"install(): XgoefInvokeResult<{ version: string; restart: boolean }>",
		`"process:exit": { args: { code?: number }; result: null }`,
		`"process:relaunch": { args: Record<string, never>; result: null }`,
		"exit(code?: number): XgoefInvokeResult<null>",
		"relaunch(): XgoefInvokeResult<null>",
		`"notification:show": { args: { title: string; body?: string; icon?: string }; result: null }`,
		"isPermissionGranted(): XgoefInvokeResult<boolean>",
		"requestPermission(): XgoefInvokeResult<string>",
		"show(opts: { title: string; body?: string; icon?: string } | string): XgoefInvokeResult<null>",
		`"global-shortcut:register": { args: { shortcut: string }; result: null }`,
		`"global-shortcut:is_registered": { args: { shortcut: string }; result: boolean }`,
		`"global-shortcut": {`,
		"register(shortcut: string): XgoefInvokeResult<null>",
		"unregister(shortcut: string): XgoefInvokeResult<null>",
		"unregisterAll(): XgoefInvokeResult<null>",
		"isRegistered(shortcut: string): XgoefInvokeResult<boolean>",
		`"monitor:list": { args: Record<string, never>; result: { id: number; scaleFactor: number; x: number; y: number; width: number; height: number; workX: number; workY: number; workWidth: number; workHeight: number; pixelX: number; pixelY: number; pixelWidth: number; pixelHeight: number; workPixelX: number; workPixelY: number; workPixelWidth: number; workPixelHeight: number; rotation: number; primary: boolean }[] }`,
		`"monitor:primary"`,
		`"monitor:scale": { args: Record<string, never>; result: number }`,
		`"monitor:move": { args: { x: number; y: number }; result: null }`,
		"list(): XgoefInvokeResult<{ id: number; scaleFactor: number;",
		"scale(): XgoefInvokeResult<number>",
		"move(x: number, y: number): XgoefInvokeResult<null>",
		`"webview:print": { args: Record<string, never>; result: null }`,
		`"webview:is_dev_tools_open": { args: Record<string, never>; result: boolean }`,
		"print(): XgoefInvokeResult<null>",
		"openDevTools(): XgoefInvokeResult<null>",
		"closeDevTools(): XgoefInvokeResult<null>",
		"isDevToolsOpen(): XgoefInvokeResult<boolean>",
		`"eval:run": { args: { name: string }; result: null }`,
		"run(name: string): XgoefInvokeResult<null>",
		`"autostart:enable": { args: Record<string, never>; result: null }`,
		`"autostart:disable": { args: Record<string, never>; result: null }`,
		`"autostart:is_enabled": { args: Record<string, never>; result: boolean }`,
		"enable(): XgoefInvokeResult<null>",
		"disable(): XgoefInvokeResult<null>",
		"isEnabled(): XgoefInvokeResult<boolean>",
		`"log:write": { args: { level?: string; message: string }; result: null }`,
		"write(level?: string, message: string): XgoefInvokeResult<null>",
		`"taskbar:skip": { args: { skip: boolean }; result: null }`,
		`"taskbar:progress": { args: { progress: number | null }; result: null }`,
		"skip(skip: boolean): XgoefInvokeResult<null>",
		"progress(progress: number | null): XgoefInvokeResult<null>",
		`"deeplink:get_current": { args: Record<string, never>; result: { urls: string[] } }`,
		"getCurrent(): XgoefInvokeResult<{ urls: string[] }>",
		`"menu:popup"`,
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("missing %s\n%s", needle, got)
		}
	}
	if strings.Contains(got, "os:") {
		t.Fatal("os was not enabled")
	}

	if _, err := Assemble([]Command{{Name: "os:platform", Result: "string"}}, []string{"os"}, false); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatal(err)
	}
	if _, err := Assemble(nil, []string{"fs", "fs"}, false); err == nil || !strings.Contains(err.Error(), "repeated") {
		t.Fatal(err)
	}
	if _, err := Assemble(nil, []string{"tray"}, false); err == nil || !strings.Contains(err.Error(), "unknown plugin") {
		t.Fatal(err)
	}
	if _, err := Normalize(Command{Name: "greet", Result: "string; evil"}); err == nil {
		t.Fatal("expected bad type")
	}
	if _, err := Normalize(Command{Name: "greet", Args: []Arg{{Name: "name", Type: "nope"}}, Result: "string"}); err == nil {
		t.Fatal("expected bad arg")
	}
	if _, err := Assemble(nil, nil, false); err != nil {
		t.Fatal(err)
	}
}

const renderWant = `// Generated by xgoef from xgoef.conf.json. Do not edit.

export interface XgoefInvokeOptions {
  timeout?: number;
  signal?: AbortSignal;
}

export interface XgoefChannel {
  readonly id: number;
  onmessage(handler: (payload: unknown) => void): () => void;
  close(): void;
}

export interface XgoefInvokeResult<T> extends Promise<T> {
  cancel(): void;
}

export interface XgoefError extends Error {
  readonly code: string;
}

export interface XgoefCommands {
  /** hello */
  greet: { args: { name?: string }; result: string };
  "os:platform": { args: Record<string, never>; result: string };
  "os:arch": { args: Record<string, never>; result: string };
  "menu:popup": { args: Record<string, never>; result: string };
  "tray:popup": { args: Record<string, never>; result: string };
  "tray:click": { args: Record<string, never>; result: string };
}

export interface Xgoef {
  invoke<K extends keyof XgoefCommands>(
    cmd: K,
    args: XgoefCommands[K]["args"],
    opts?: XgoefInvokeOptions,
  ): XgoefInvokeResult<XgoefCommands[K]["result"]>;
  listen(event: string, handler: (payload: unknown) => void): () => void;
  emit(event: string, payload?: unknown): void;
  channel(): XgoefChannel;
  readonly XgoefError: new (code: string, message?: string) => XgoefError;
  plugins: {
    os: {
      platform(): XgoefInvokeResult<string>;
      arch(): XgoefInvokeResult<string>;
    };
  };
}

declare global {
  const __XGOEF__: Xgoef;
  interface Window {
    __XGOEF__: Xgoef;
    __xgoefDispatch(msg: unknown): void;
  }
}
`
