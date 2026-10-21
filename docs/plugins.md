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
| `process` | `process:exit` `process:relaunch` | 与命令名相同 |
| `notification` | `notification:is_permission_granted` `notification:request_permission` `notification:show` | 与命令名相同 |

前端：`__XGOEF__.plugins.fs.readText(path)`、`__XGOEF__.plugins.os.platform()`、`plugins.shell.open`、`plugins.clipboard.readText` / `writeText`、`plugins.dialog.open` / `save`、`plugins.updater.check` / `install`、`plugins.process.exit(code)` / `relaunch()`、`plugins.notification.requestPermission()` / `show(title)`。启用的插件会写进 `web/xgoef.d.ts`，见 [binding.md](binding.md)。

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

`..` 和指向范围外的符号链接都会拒绝，错误码是 `fs.denied`。hello 用 `web/note.txt` 演示 `fs:read_text`。

## shell

`shell:open` 接受 `url` 或 `path`，不会经过 shell。

- URL 默认只允许 `http` 和 `https`，拒绝用户名密码、`file:` 和 `javascript:`。这些拒绝的错误码是 `shell.denied`。
- 本地路径要单独放进 `plugin.shell.allow`。没配 allow 就拒绝所有路径，错误码同样是 `shell.denied`。
- 实际启动用 `xdg-open`，参数是一个 argv，不是命令字符串。

## clipboard

默认是进程内文本剪贴板（`plugin.clipboard.mode` 为 `memory`）。`system` 改用 `wl-copy`/`wl-paste` 或 `xclip`。文本上限 1 MiB。hello 的 *clipboard* 按钮走 memory。

## dialog

`dialog:open` / `dialog:save` 先弹出选择器，再把返回的路径放进 `plugin.dialog.allow`。选到范围外是 `dialog.denied`。取消返回 `null`。默认选择器是 `zenity`；没装就报错，不会退回任意路径。没有单独记号的失败在页面上是 `command.failed`。见 [ipc.md](ipc.md)。

## updater

`updater:check` / `updater:install` 读取 `plugin.updater.endpoint` 上的 `update.json` 和 `xgoef.assets`。公钥是 `build.assetPublicKey`。细节见 [updater.md](updater.md)。

## process

`process:exit` 结束当前进程。`code` 省略时是 0，只接受 0 到 255 的整数，其它值是 `process.invalid`。进程在响应写回之前就结束，页面不一定收得到结果。

`process:relaunch` 再启动当前宿主。参数沿用这次启动的参数，并去掉 CEF 子进程的 `--type` 和 `--utility-sub-type`。页面不能改可执行文件，也不能附加参数；带了参数就是 `process.invalid`。单实例锁先放开，再启动新进程，然后当前进程以 0 退出。可执行文件不存在时不会放开锁。启动失败是 `process.relaunch`。hello 的 *exit* 和 *relaunch* 走这两条命令。

## notification

`notification:request_permission` 返回 `granted` 或 `denied`。桌面有 `notify-send` 时授予，否则拒绝。这个状态只在当前进程里，启动时还没授予。`notification:is_permission_granted` 返回布尔值。

`notification:show` 在授予之后发出通知。参数是 `title`，可选 `body` 和 `icon`。没授予是 `notification.denied`，不会去显示。标题必须是一行，最长 256 字节；正文最长 4096 字节。不合规则是 `notification.invalid`。

`icon` 是范围内的文件，不是图标主题名。空的 `allow` 仍然可以只发标题和正文；给了范围外的路径是 `notification.icon`。实际调用是 `notify-send` 的 argv，标题放在 `--` 后面，不经过 shell。应用名来自 `productName`。hello 的 *notify* 先请求权限，再显示标题 xgoef。

## 应用侧还要做的

1. `capabilities[].permissions` 写上要用的命令名（`core:default` 不会放行它们）。
2. 在 `Run` 之前 `app.SetBase(配置目录)`，否则 `$APP` 取当前工作目录。
