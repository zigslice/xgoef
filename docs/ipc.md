# IPC（P0）

前端 `invoke` → CEF MessageRouter → `runtime` → `ipc.Router`。

## 消息约定

```json
{ "type": "invoke", "id": 1, "cmd": "greet", "args": { "name": "xgo" } }
{ "type": "response", "id": 1, "ok": true, "payload": "hello, xgo (from xgoef)" }
{ "type": "response", "id": 1, "ok": false, "error": "acl: ..." }
```

## 传输

1. 渲染进程：`CefMessageRouterRendererSide` 在 `OnContextCreated` 注入 `window.cefQuery`。
2. `api/js/xgoef.js` 把 invoke JSON 作为 `cefQuery({ request })`。
3. 浏览器进程：`IpcHandler::OnQuery` → `xgoef_ipc_query_fn`（Go）→ ACL → `Router.Invoke` → `xgoef_ipc_respond` → `Callback::Success`（响应用 JSON envelope，便于 Promise reject）。

Event 见 [events.md](events.md)。有序的宿主 → 页面流见 [channels.md](channels.md)。

## C ABI

见 `cef/shim/xgoef_host.h`：`xgoef_ipc_set_handler` / `xgoef_ipc_respond`。
