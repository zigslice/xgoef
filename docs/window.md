# Window API（P1）

```text
App
 └─ Window (label, title, bounds, url)
      └─ WebView (CEF browser)
```

## Host API

| Method | Behavior |
| --- | --- |
| `CreateWindow(WindowSpec)` | Opens another Views window; binds `label` |
| `CloseWindow(label)` | Closes the browser for that label |
| `FocusWindow(label)` | Shows / activates the window |
| `EmitTo` / `Emit` | See [events.md](events.md) |

`WindowSpec`: `Label`, `Title`, `URL`, `Width`, `Height`.

## Conf

`app.windows[].label` (default `main`) is applied via `--xgoef-label=` on startup.
Capabilities should list each label that may invoke commands.

窗口菜单和托盘见 [menu.md](menu.md)。

## 记住位置（window-state）

默认会把每个窗口的屏幕坐标、宽高和是否最大化写到应用目录的 `.xgoef/window-state.json`。下次启动按 label 恢复。最大化时仍保留恢复前的宽高，避免把铺满屏幕的尺寸当成普通尺寸。最小化不写入，下次也不会以最小化启动。

`app.windowState` 设为 `false` 就停用。省略这个字段时是开启的。

第一扇窗口的尺寸来自 `--xgoef-size=宽,高`。有保存记录时用记录里的宽高，否则用配置里的 `width` / `height`。再往后 `CreateWindow` 同样优先用保存的宽高；坐标和最大化由 shim 在创建 Views 窗口时套用。

移动、缩放或关闭已经绑定 label 的窗口时，shim 回调宿主写文件。坏掉的 JSON 不会拦住启动，下一次成功记住时整文件重写。

这份状态只走 Views 窗口。`--use-native` 那条路径不读它。

传给 shim 的文本每个窗口一行，六列用制表符分开：`label`、`x`、`y`、`width`、`height`、`flags`。`p` 表示坐标和尺寸都要套用，`m` 表示最大化。两项都在时写成 `pm`。`cef/shim/xgoef_window_state.cc` 按同一规则解析，且不依赖 CEF 头文件。
