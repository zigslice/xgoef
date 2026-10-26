package binding

// builtinPlugins are the commands each builtin registers.
// Field order matches the positional helpers in xgoef.js, except dialog
// and shell, which take one value.
var builtinPlugins = map[string][]Command{
	"fs": {
		{Name: "fs:read_text", Args: []Arg{{Name: "path", Type: "string"}}, Result: "string"},
		{Name: "fs:write_text", Args: []Arg{{Name: "path", Type: "string"}, {Name: "contents", Type: "string"}}, Result: "null"},
		{Name: "fs:exists", Args: []Arg{{Name: "path", Type: "string"}}, Result: "boolean"},
		{Name: "fs:list", Args: []Arg{{Name: "path", Type: "string"}}, Result: "{ name: string; isDir: boolean }[]"},
	},
	"os": {
		{Name: "os:platform", Result: "string"},
		{Name: "os:arch", Result: "string"},
	},
	"shell": {
		{Name: "shell:open", Args: []Arg{{Name: "url", Type: "string", Optional: true}, {Name: "path", Type: "string", Optional: true}}, Result: "string"},
	},
	"clipboard": {
		{Name: "clipboard:read_text", Result: "string"},
		{Name: "clipboard:write_text", Args: []Arg{{Name: "text", Type: "string"}}, Result: "null"},
	},
	"dialog": {
		{
			Name: "dialog:open",
			Args: []Arg{
				{Name: "title", Type: "string", Optional: true},
				{Name: "defaultPath", Type: "string", Optional: true},
				{Name: "directory", Type: "boolean", Optional: true},
			},
			Result: "string | null",
		},
		{
			Name: "dialog:save",
			Args: []Arg{
				{Name: "title", Type: "string", Optional: true},
				{Name: "defaultPath", Type: "string", Optional: true},
				{Name: "directory", Type: "boolean", Optional: true},
			},
			Result: "string | null",
		},
	},
	"updater": {
		{Name: "updater:check", Result: "{ version: string; notes?: string; current: string } | null"},
		{Name: "updater:install", Result: "{ version: string; restart: boolean }"},
	},
	"process": {
		{Name: "process:exit", Args: []Arg{{Name: "code", Type: "number", Optional: true}}, Result: "null"},
		{Name: "process:relaunch", Result: "null"},
	},
	"notification": {
		{Name: "notification:is_permission_granted", Result: "boolean"},
		{Name: "notification:request_permission", Result: "string"},
		{
			Name: "notification:show",
			Args: []Arg{
				{Name: "title", Type: "string"},
				{Name: "body", Type: "string", Optional: true},
				{Name: "icon", Type: "string", Optional: true},
			},
			Result: "null",
		},
	},
	"global-shortcut": {
		{Name: "global-shortcut:register", Args: []Arg{{Name: "shortcut", Type: "string"}}, Result: "null"},
		{Name: "global-shortcut:unregister", Args: []Arg{{Name: "shortcut", Type: "string"}}, Result: "null"},
		{Name: "global-shortcut:unregister_all", Result: "null"},
		{Name: "global-shortcut:is_registered", Args: []Arg{{Name: "shortcut", Type: "string"}}, Result: "boolean"},
	},
	"monitor": {
		{Name: "monitor:list", Result: displayType + "[]"},
		{Name: "monitor:primary", Result: displayType + " | null"},
		{Name: "monitor:current", Result: displayType + " | null"},
		{Name: "monitor:scale", Result: "number"},
		{Name: "monitor:move", Args: []Arg{{Name: "x", Type: "number"}, {Name: "y", Type: "number"}}, Result: "null"},
	},
}

// displayType is one monitor. DIP fields match window-state; pixel fields are physical.
const displayType = "{ id: number; scaleFactor: number; x: number; y: number; width: number; height: number; workX: number; workY: number; workWidth: number; workHeight: number; pixelX: number; pixelY: number; pixelWidth: number; pixelHeight: number; workPixelX: number; workPixelY: number; workPixelWidth: number; workPixelHeight: number; rotation: number; primary: boolean }"

var chromeCommands = []Command{
	{Name: "menu:popup", Result: "string"},
	{Name: "tray:popup", Result: "string"},
	{Name: "tray:click", Result: "string"},
}

var deepLinkCommand = Command{
	Name:   "deeplink:get_current",
	Result: "{ urls: string[] }",
}

// helperNS is the plugins.* object in xgoef.js.
var helperNS = []string{"fs", "os", "shell", "clipboard", "dialog", "updater", "process", "notification", "global-shortcut", "monitor", "deeplink"}
