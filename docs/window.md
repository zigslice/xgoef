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

窗口菜单和托盘见 [menu.md](menu.md)。位置、尺寸和最大化在关闭时还不写回配置。
