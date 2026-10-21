# 插件（P1）

插件 = 一组 IPC 命令 + 对应的 capability 名。运行时默认拒绝：窗口的 `permissions` 里没有该命令名，调用就失败。

## 契约

```go
type Plugin interface {
    Name() string
    Permissions() []string
    Setup(app plugin.Host) error
    Teardown(app plugin.Host) error
}
```

`Setup` 里 `app.Register(name, handler)`。`*runtime.App` 实现 `plugin.Host`。

两种装载方式：

- 配置：`"plugins": ["fs", "os"]`，`App.Run` → `OpenPlugins`
- 代码：`app.Use(p)`（同名再装一次会报错）

## 内置

| id | 命令 | 权限 |
| --- | --- | --- |
| `fs` | `fs:read_text` `fs:write_text` `fs:exists` `fs:list` | 与命令名相同 |
| `os` | `os:platform` `os:arch` | 与命令名相同 |
| `shell` | `shell:open` | 与命令名相同 |
| `clipboard` | `clipboard:read_text` `clipboard:write_text` | 与命令名相同 |
| `dialog` | `dialog:open` `dialog:save` | 与命令名相同 |
| `updater` | `updater:check` `updater:install` | 与命令名相同 |

前端：`__XGOEF__.plugins.fs.readText(path)`、`__XGOEF__.plugins.os.platform()`、`plugins.shell.open`、`plugins.clipboard.readText` / `writeText`、`plugins.dialog.open` / `save`、`plugins.updater.check` / `install`。启用的插件会写进 `web/xgoef.d.ts`，见 [binding.md](binding.md)。

文本读写上限 1 MiB。

## fs 范围

空的 `allow` 拒绝所有路径。`deny` 优先于 `allow`。

```json
"plugins": ["fs", "os"],
"plugin": {
  "fs": {
    "allow": ["$APP", "$RESOURCE"],
    "deny": ["$APP/secret"]
  }
}
```

| 根 | 含义 |
| --- | --- |
| `$APP` | `App.SetBase` 的目录（hello 里是配置文件所在目录） |
| `$RESOURCE` | `build.frontendDist`，相对 `$APP` |
| `$HOME` | 用户主目录 |
| 其他 | 绝对路径，或相对 `$APP` |

`..` 和指向范围外的符号链接都会拒绝。hello 用 `web/note.txt` 演示 `fs:read_text`。

## shell

`shell:open` 接受 `url` 或 `path`，不会经过 shell。

- URL 默认只允许 `http` 和 `https`，拒绝用户名密码、`file:` 和 `javascript:`。
- 本地路径要单独放进 `plugin.shell.allow`。没配 allow 就拒绝所有路径。
- 实际启动用 `xdg-open`，参数是一个 argv，不是命令字符串。

## clipboard

默认是进程内文本剪贴板（`plugin.clipboard.mode` 为 `memory`）。`system` 改用 `wl-copy`/`wl-paste` 或 `xclip`。文本上限 1 MiB。hello 的 *clipboard* 按钮走 memory。

## dialog

`dialog:open` / `dialog:save` 先弹出选择器，再把返回的路径放进 `plugin.dialog.allow`。取消返回 `null`。默认选择器是 `zenity`；没装就报错，不会退回任意路径。

## updater

`updater:check` / `updater:install` 读取 `plugin.updater.endpoint` 上的 `update.json` 和 `xgoef.assets`。公钥是 `build.assetPublicKey`。细节见 [updater.md](updater.md)。

## 应用侧还要做的

1. `capabilities[].permissions` 写上要用的命令名（`core:default` 不会放行它们）。
2. 在 `Run` 之前 `app.SetBase(配置目录)`，否则 `$APP` 取当前工作目录。
