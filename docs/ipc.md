# IPC（P0）

前端 `invoke` → CEF MessageRouter → `runtime` → `ipc.Router`。

## 消息约定

```json
{ "type": "invoke", "id": 1, "cmd": "greet", "args": { "name": "xgo" }, "timeout": 2000 }
{ "type": "cancel", "invoke": 1 }
{ "type": "response", "id": 1, "ok": true, "payload": "hello, xgo (from xgoef)" }
{ "type": "response", "id": 1, "ok": false, "error": "ipc: canceled" }
```

`timeout` 是毫秒，正整数。缺省或 `0` 表示这次调用自己不设期限。负数、小数，或超过 24 小时，返回 `ipc: invalid timeout`。

## 传输

1. 渲染进程：`CefMessageRouterRendererSide` 在 `OnContextCreated` 注入 `window.cefQuery`。
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

## C ABI

见 `cef/shim/xgoef_host.h`：`xgoef_ipc_set_handler` / `xgoef_ipc_respond` / `xgoef_ipc_set_cancel_handler`。
