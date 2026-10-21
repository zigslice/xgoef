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
}

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
var helperNS = []string{"fs", "os", "shell", "clipboard", "dialog", "updater", "process", "deeplink"}
