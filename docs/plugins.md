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
| `global-shortcut` | `global-shortcut:register` `global-shortcut:unregister` `global-shortcut:unregister_all` `global-shortcut:is_registered` | 与命令名相同 |
| `monitor` | `monitor:list` `monitor:primary` `monitor:current` `monitor:scale` `monitor:move` | 与命令名相同 |
| `webview` | `webview:print` `webview:open_dev_tools` `webview:close_dev_tools` `webview:is_dev_tools_open` | 与命令名相同 |
| `eval` | `eval:run` | 与命令名相同 |
| `autostart` | `autostart:enable` `autostart:disable` `autostart:is_enabled` | 与命令名相同 |
| `log` | `log:write` | 与命令名相同 |
| `window` | `window:minimize` `window:maximize` `window:unminimize` `window:unmaximize` `window:toggle_maximize` `window:show` `window:hide` `window:center` `window:set_title` `window:set_always_on_top` `window:set_fullscreen` `window:set_size` `window:is_minimized` `window:is_maximized` `window:is_fullscreen` `window:is_always_on_top` `window:is_visible` `window:set_resizable` `window:is_resizable` `window:set_minimizable` `window:is_minimizable` `window:set_maximizable` `window:is_maximizable` `window:set_closable` `window:is_closable` `window:set_min_size` `window:set_max_size` `window:min_size` `window:max_size` | 与命令名相同 |

前端：`__XGOEF__.plugins.fs.readText(path)`、`__XGOEF__.plugins.os.platform()`、`plugins.shell.open`、`plugins.clipboard.readText` / `writeText`、`plugins.dialog.open` / `save`、`plugins.updater.check` / `install`、`plugins.process.exit(code)` / `relaunch()`、`plugins.notification.requestPermission()` / `show(title)`、`plugins["global-shortcut"].register(shortcut)`、`plugins.monitor.list()` / `scale()` / `move(x, y)`、`plugins.webview.print()` / `openDevTools()` / `closeDevTools()` / `isDevToolsOpen()`、`plugins.eval.run(name)`、`plugins.autostart.enable()` / `disable()` / `isEnabled()`、`plugins.log.trace(message)` / `debug` / `info` / `warn` / `error`、`plugins.window.minimize()` / `toggleMaximize()` / `setTitle(title)` / `setSize(width, height)` / `isMaximized()` / `setResizable(resizable)` / `setMinSize(width, height)` / `minSize()`。启用的插件会写进 `web/xgoef.d.ts`，见 [binding.md](binding.md)。

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

## global-shortcut

`global-shortcut:register` 注册一条全局快捷键。字符串像 `Control+Shift+X`，大小写不敏感，`+` 两边可以有空格。至少要有一个修饰键（Control、Alt、Shift、Super），避免把单个字母从输入里抢走。`CommandOrControl` 在 Linux 上是 Control，在 macOS 上是 Super。规范形式固定为 Control、Alt、Shift、Super 再加键名，所以 `ctrl+shift+x` 和 `Shift+Control+X` 是同一条。

空的 `plugin.global-shortcut.allow` 拒绝每一次注册。不在列表里是 `shortcut.denied`。写法不合法是 `shortcut.invalid`。已经注册过是 `shortcut.registered`。

按下和松开时宿主发出事件 `global-shortcut`，payload 是 `{shortcut, state}`。`state` 是 `Pressed` 或 `Released`。页面用 `__XGOEF__.listen` 接收。

Linux 上用 X11 的 `XGrabKey`，需要 `DISPLAY`（X11 或 XWayland）。没有显示器是 `shortcut.unavailable`。抓键失败是 `shortcut.grab`。NumLock 或 CapsLock 打开时仍然匹配。只通过 XWayland 暴露 `DISPLAY` 的 Wayland 会话里，合成器可能把全局键留在自己这边，页面就收不到按下事件。参数不经过 shell。

`unregister` 取消一条，没注册过也成功。`unregister_all` 取消这个进程注册过的全部。`is_registered` 返回布尔值。前端是 `plugins["global-shortcut"].register(shortcut)`、`unregister`、`unregisterAll`、`isRegistered`。hello 的 *shortcut* 注册 `Control+Shift+X`。

## monitor

`monitor:list` 返回每块显示器的 DIP 边界、工作区、物理像素、`scaleFactor` 和是否主屏。`monitor:primary` 是主屏。`monitor:current` 是发出调用的窗口所在的屏。`monitor:scale` 是这个窗口的 device scale。`monitor:move` 把这个 Views 窗口的左上角移到 DIP 坐标。坐标和 window-state 是同一套。细节和错误码见 [window.md](window.md)。hello 的 *monitors* 按钮打印显示器数量和当前 scale。

## webview

`webview:print` 对发出调用的窗口调用 CEF `Print()`，弹出系统打印对话框。用户在对话框里确认。这条命令不写 PDF。

`webview:open_dev_tools`、`webview:close_dev_tools` 和 `webview:is_dev_tools_open` 开关并查询开发者工具。配置 `plugin.webview.devtools` 省略或为 false 时，这三条返回 `webview.denied`，不会调用 CEF。打印不看这个开关。hello 把它设为 true，*devtools* 按钮在打开和关闭之间切换。

开发者工具那扇浏览器不进入窗口 label，收不到宿主事件，也不会在最后一扇应用窗口关掉之后把进程留住。细节见 [window.md](window.md)。

## eval

`eval:run` 只接受 `{name}`。名字对应 `plugin.eval.scripts` 里的一段脚本。页面不能提交脚本正文，也不能带上别的字段或窗口 label。多出来的字段是 `eval.denied`，这次调用不会执行。没登记的名字同样是 `eval.denied`。

脚本只在发出调用的那个窗口的主框架里执行，而且这个框架的地址必须还在该窗口钉住的源站上。离开 `xgoef://app` 或配置里的 `http(s)` 源站之后，调用是 `eval.denied`，CEF 不会执行。`javascript:`、`data:`、`file:` 和 `about:blank` 不能当源站。同一窗口里的子框架如果能发出 invoke，权限跟这个窗口算在一起，但脚本仍然只写进主框架。不要把不受信任的页面嵌进应用窗口。

宿主代码可以调用 `App.Eval(label, source)`。`source` 必须是宿主自己写的文本，不能把页面传来的字符串传进去。页面侧的入口只有名字。

注入脚本写在窗口的 `initScripts` 里，不走这条命令。细节见 [window.md](window.md)。hello 的 *eval* 按钮运行名字 `mark`。

## autostart

`autostart:enable` 在当前用户的 XDG autostart 目录写入 `xgoef-<identifier>.desktop`。`autostart:disable` 删除这个文件。`autostart:is_enabled` 返回布尔值。进程启动时不会自己写入。

`Exec` 使用宿主绝对路径和 `plugin.autostart.args`。页面不能改可执行文件，也不能附加参数；带了字段就是 `autostart.invalid`。Linux 以外是 `autostart.unavailable`。目标若是符号链接，`autostart.denied`，不会跟着链接去写。细节见 [autostart.md](autostart.md)。hello 的 *autostart* 按钮在打开和关闭之间切换。

## log

`log:write` 接受 `{level, message}`。`level` 可以省略，省略是 `info`。低于 `plugin.log.level` 的调用成功，但不写文件。消息必须是一行，不能含有控制字符，最长 4096 字节。多出来的字段是 `log.denied`。页面不能选择目录或文件名，也不能提交崩溃记录。

Linux 上日志在 `$XDG_STATE_HOME/xgoef/xgoef-<identifier>.log`，没有这个变量时在 `~/.local/state/xgoef`。宿主 panic 另写 `xgoef-<identifier>.crash`，只留最近一次，并在日志里追加一行 `CRASH`。启用插件后，浏览器进程带上 `--xgoef-crash-dumps`，再转成 Chromium 的 crashpad 开关。dump 本身要有 `chrome_crashpad_handler`。细节见 [log.md](log.md)。hello 的 *log* 按钮写一条 info。

## window

`window` 的命令作用在发出调用的那一扇 Views 窗口上。页面不能在参数里指定另一个 label。最小化、最大化、还原、显示、隐藏、居中、标题、置顶、全屏、尺寸和对应的查询都在这里。把窗口左上角移走用 `monitor:move`。

`set_title` 的 `title` 可以是空字符串，必须是 UTF-8，最长 256 字节，不能含有小于 `0x20` 的字节。`set_size` 的宽和高是 1 到 16384 的整数。多出来的字段是 `window.invalid`。没有 Views 窗口时是 `window.unavailable`，`--use-native` 就是这种情况。细节见 [window.md](window.md)。hello 的 *window* 按钮在最大化和还原之间切换，并显示当前是不是最大化。

`set_resizable`、`set_minimizable`、`set_maximizable`、`set_closable` 各带一个布尔。对应的 `is_*` 返回布尔。`set_min_size` 和 `set_max_size` 用 `{width,height}`，`0,0` 取消这一侧的限制。`min_size` 和 `max_size` 在没设时返回 `null`。下限不能大于上限。配置里的同名窗口字段会在创建前写进 shim。hello 的 *limits* 按钮把最小尺寸设成 320×240 并读回来。

## 应用侧还要做的

1. `capabilities[].permissions` 写上要用的命令名（`core:default` 不会放行它们）。
2. 在 `Run` 之前 `app.SetBase(配置目录)`，否则 `$APP` 取当前工作目录。
