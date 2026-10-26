# IPC（P0）

前端 `invoke` → CEF MessageRouter → `runtime` → `ipc.Router`。

## 消息约定

```json
{ "type": "invoke", "id": 1, "cmd": "greet", "args": { "name": "xgo" }, "timeout": 2000 }
{ "type": "cancel", "invoke": 1 }
{ "type": "response", "id": 1, "ok": true, "payload": "hello, xgo (from xgoef)" }
{ "type": "response", "id": 1, "ok": false, "error": { "code": "ipc.canceled", "message": "ipc: canceled" } }
```

`timeout` 是毫秒，正整数。缺省或 `0` 表示这次调用自己不设期限。负数、小数，或超过 24 小时，返回 `ipc: invalid timeout`。

## 传输

1. 渲染进程：`CefMessageRouterRendererSide` 在 `OnContextCreated` 注入 `window.cefQuery`，随后执行前端根里的 `xgoef.js`（见下一节）。
2. `api/js/xgoef.js` 把 invoke JSON 作为 `cefQuery({ request })`。
3. 浏览器进程：`IpcHandler::OnQuery` → `xgoef_ipc_query_fn`（Go）→ ACL。通过后命令在回调外执行，完成时 `xgoef_ipc_respond` → `Callback::Success`（响应用 JSON envelope，便于 Promise reject）。
4. 页面发 `cancel`，或 CEF 在导航时调用 `OnQueryCanceled`，这条调用的 context 结束。期限到了也结束。调用方马上收到 `ipc: canceled` 或 `ipc: timeout`。

Event 见 [events.md](events.md)。有序的宿主 → 页面流见 [channels.md](channels.md)。

## 异步、超时与取消

页面上的 command 不再占着 CEF UI 线程。这次调用的 `ctx.Done()` 会在取消或超时后关闭。菜单点击和 `Router.Invoke` 仍在调用方里面跑完，用的 context 不会结束，`Done()` 为 nil。

```js
const job = window.__XGOEF__.invoke("wait", {}, { timeout: 2000 })
job.cancel()
await job
```

`opts.signal` 是 `AbortSignal` 时，`abort` 会取消同一次调用。`App.SetInvokeTimeout` 给没有更短期限的页面调用加一个上限，两边都有时用更短的那个。

```xgo
app.Handle "wait", (ctx, argsJSON) => {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case <-time.After(1500 * time.Millisecond):
        return "waited", nil
    }
}
```

Handler 看 `ctx.Done()` 才会停下来。不看的 handler 会继续跑到返回，但页面已经按取消或超时结束，晚到的结果不会再写回去。取消后的 context 错误是 `ipc: canceled`，期限到了是 `ipc: timeout`。

hello 的 *wait* 停 1.5 秒，*cancel wait* 取消它。

`xgoef build` / `xgoef dev` 会按配置写出 `web/xgoef.d.ts`，见 [binding.md](binding.md)。

## 结构化错误

失败时 `error` 是对象。`code` 是稳定记号，`message` 是给人看的那句文本。

页面上 `invoke` reject 的是 `XgoefError`（`__XGOEF__.XgoefError`）。`e.code` 用来分支，`e.message` 是上面的文本。声明见 [binding.md](binding.md)。

```xgo
return nil, ipc.Fail("greet.empty", "name is empty")
```

记号是小写段，用点分开，至少两段，最长 64 个字符，例如 `greet.empty`。不合语法的记号会变成 `command.failed`，message 仍保留。普通 `error` 也是 `command.failed`。

| code | 何时 |
| --- | --- |
| `ipc.canceled` | 页面取消，或导航丢掉这次调用 |
| `ipc.timeout` | 超过期限 |
| `ipc.invalid_timeout` | `timeout` 不是合法毫秒 |
| `ipc.unknown_command` | 没有这个 command |
| `ipc.missing_cmd` | 没写 `cmd` |
| `ipc.invalid_message` | invoke JSON 解析失败 |
| `ipc.no_router` | 没有 router |
| `ipc.panic` | handler panic |
| `ipc.bad_response` | 返回值无法编码，或页面解析不了宿主响应 |
| `ipc.unsupported` | 不认识的消息 type |
| `ipc.missing_channel` | 没有 channel id |
| `ipc.channel_closed` | channel 已关 |
| `ipc.channel_args` | channel 参数不是对象 |
| `ipc.channel_count` | 参数里不是恰好一个 channel |
| `ipc.channels_unconfigured` | 宿主没接 channel |
| `ipc.encode` | 响应本身编码失败 |
| `ipc.disconnected` | 页面上没有 `cefQuery` / `__xgoefPost` |
| `ipc.transport` | `cefQuery` 失败 |
| `acl.denied` | capability 拒绝，见 [acl.md](acl.md) |
| `fs.denied` / `shell.denied` / `dialog.denied` | 路径或 URL 不在范围内 |
| `process.invalid` | `process:exit` 的退出码不合法，或 `process:relaunch` 带了参数 |
| `process.relaunch` | 重启宿主失败 |
| `notification.invalid` | 通知标题、正文或参数不合法 |
| `notification.denied` | 还没有通知权限 |
| `notification.icon` | 图标不在配置的目录范围内 |
| `notification.show` | `notify-send` 失败 |
| `shortcut.invalid` | 快捷键字符串或参数不合法 |
| `shortcut.denied` | 快捷键不在 `plugin.global-shortcut.allow` 里 |
| `shortcut.registered` | 这条快捷键已经注册过 |
| `shortcut.unavailable` | 当前进程装不上全局抓键 |
| `shortcut.grab` | 向系统注册或取消抓键失败 |
| `monitor.invalid` | 移动坐标不是范围内的整数 |
| `monitor.unavailable` | 读不到显示器，或这个 label 没有 Views 窗口可移动 |
| `webview.denied` | `plugin.webview.devtools` 不是 true，开发者工具命令不会调用 CEF |
| `webview.unavailable` | 这个 label 没有浏览器，或打印 / 开发者工具调用失败 |
| `eval.invalid` | `eval:run` 没有合法的 `name`，或脚本正文被宿主拒绝 |
| `eval.denied` | 参数里除了名字还有别的字段，名字没有登记，或窗口已经离开钉住的源站。这种失败不会执行脚本 |
| `eval.unavailable` | 这个 label 没有浏览器，或执行调用没有完成 |
| `command.failed` | 没有自己记号的 handler 错误 |

## 自动注入

`xgoef-helper` 在 `OnContextCreated` 里先让 MessageRouter 装上 `window.cefQuery`，再读 `--xgoef-asset-root` 目录中的 `xgoef.js`，用 V8 `Eval` 执行。页面自己的脚本开始之前，`window.__XGOEF__` 已经在。这是宿主侧执行，不走页面的 `<script>`，页面 CSP 拦不住它。

主框架会注入。`devtools://`、`view-source:` 和以 `chrome` 开头的内部地址不注入。子框架只在和主框架同源时注入：`xgoef://app/` 下的 iframe，以及 dev server 同一 origin 的 iframe。其它 origin 没有 `__XGOEF__`。`about:blank` 子框架不注入。

`api/js/xgoef.js` 若发现 `__XGOEF__` 已经存在就返回。页面可以继续写 `<script src="xgoef.js"></script>`，第二次执行不会清掉已经注册的 `listen`。

`app.withGlobalXgoef` 省略或为 `true` 时开启。设为 `false` 时，宿主带上 `--xgoef-no-bridge`，渲染进程不注入，页面必须自己引用脚本。这对应 Tauri 的 `withGlobalTauri`，这里默认开启。

`xgoef build` 和 `xgoef dev` 仍把桥复制到前端根。渲染进程每次创建上下文都重新读这份文件。浏览器进程在 `OnBeforeChildProcessLaunch` 把 `--xgoef-asset-root` 和 `--xgoef-no-bridge` 传给子进程。文件缺失、不是普通文件、超过 1MiB，或执行抛错时，向 stderr 打一行 `xgoef:`，页面上就没有桥。

命令仍要过 capabilities。注入只是把 API 放进页面。

## C ABI

见 `cef/shim/xgoef_host.h`：`xgoef_ipc_set_handler` / `xgoef_ipc_respond` / `xgoef_ipc_set_cancel_handler`。
