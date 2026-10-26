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

`WindowSpec`: `Label`, `Title`, `URL`, `Width`, `Height`, `Chrome`。`Chrome` 的零值是有装饰、跟随系统、不透明。

## Conf

`app.windows[].label` (default `main`) is applied via `--xgoef-label=` on startup.
Capabilities should list each label that may invoke commands.

窗口菜单和托盘见 [menu.md](menu.md)。第二次启动同一 identifier 时会交给已经在跑的进程，见 [instance.md](instance.md)。

## 记住位置（window-state）

默认会把每个窗口的屏幕坐标、宽高和是否最大化写到应用目录的 `.xgoef/window-state.json`。下次启动按 label 恢复。最大化时仍保留恢复前的宽高，避免把铺满屏幕的尺寸当成普通尺寸。最小化不写入，下次也不会以最小化启动。

`app.windowState` 设为 `false` 就停用。省略这个字段时是开启的。

第一扇窗口的尺寸来自 `--xgoef-size=宽,高`。有保存记录时用记录里的宽高，否则用配置里的 `width` / `height`。再往后 `CreateWindow` 同样优先用保存的宽高；坐标和最大化由 shim 在创建 Views 窗口时套用。

移动、缩放或关闭已经绑定 label 的窗口时，shim 回调宿主写文件。坏掉的 JSON 不会拦住启动，下一次成功记住时整文件重写。

这份状态只走 Views 窗口。`--use-native` 那条路径不读它。

传给 shim 的文本每个窗口一行，六列用制表符分开：`label`、`x`、`y`、`width`、`height`、`flags`。`p` 表示坐标和尺寸都要套用，`m` 表示最大化。两项都在时写成 `pm`。`cef/shim/xgoef_window_state.cc` 按同一规则解析，且不依赖 CEF 头文件。

## 主题、装饰与透明

每个窗口可以写这三项。省略时分别是有装饰、跟随系统、不透明。

| 字段 | 含义 |
| --- | --- |
| `decorations` | `false` 时 Views 窗口走 `IsFrameless`，标题栏和边框隐藏，窗口仍可缩放。`--use-native` 没有 Views delegate，这条不生效。 |
| `theme` | `system`、`light`、`dark`。大小写不敏感，读入后记成小写。别的值让 `conf.Load` 失败，错误形如 `conf: window main theme "blue"`。 |
| `transparent` | `true` 把浏览器背景设成全透明（`background_color` 为 0）。窗口化 CEF 会把它画成不透明白，桌面不会透出来。没有做离屏渲染。 |

第一扇窗口把这三项编成 `--xgoef-chrome=装饰,主题,透明`，例如 `1,system,0` 或 `0,dark,1`。`dark` 再加进程级的 `--force-dark-mode`，`light` 再加 `--force-light-mode`，用来定原生边框的初始主题。这两个开关只跟第一扇窗口走，后面窗口的原生边框仍跟着它。`system` 不加。

`light` 和 `dark` 还会给这扇窗口单独的 `CefRequestContext`（与全局 context 共享存储），调用 `SetChromeColorScheme`，所以不会把别的窗口一起换色。`system` 不调用它，request context 仍是空。`window.open` 弹出的窗口沿用父窗口的装饰，配色则跟父窗口的 request context。

再 `CreateWindow` 时，`WindowSpec.Chrome` 用同一段文本作为 `xgoef_window_create` 的最后一项。`cef/shim/xgoef_window_chrome.cc` 按同一规则解析，且不依赖 CEF 头文件。坏文本打印到 stderr 并退回默认，C ABI 仍返回 0。

页面背景：透明为 0；`dark` 为 `0xFF1E1E1E`；`light` 为 `0xFFFFFFFF`；`system` 留给 CEF 默认。

## 多显示器与 DPI

`monitor` 插件列出 CEF 看见的显示器，并可以移动当前 Views 窗口。坐标 `x` / `y` / `width` / `height` 和 `workX` / `workY` / `workWidth` / `workHeight` 是 DIP，和 window-state 里记住的屏幕坐标是同一套。`scaleFactor` 是这块屏的 device scale。`pixelX` 这一组是 CEF `ConvertPointToPixels` 算出来的物理像素。`rotation` 是 0、90、180 或 270。CEF 不提供显示器的名字。

| 命令 | 返回 |
| --- | --- |
| `monitor:list` | 每块屏一条 |
| `monitor:primary` | 主屏，没有显示器时是 `null` |
| `monitor:current` | 调用窗口所在的屏。这个 label 没有 Views 窗口时是 `null` |
| `monitor:scale` | 调用窗口所在屏的 `scaleFactor`。窗口还没有屏时用主屏的。一块屏都没有是 `monitor.unavailable` |
| `monitor:move` | 把调用窗口的左上角移到 DIP `x`, `y`，宽高不变 |

`--use-native` 没有 Views 窗口，`monitor:current` 是 `null`，`monitor:move` 是 `monitor.unavailable`。`x` 和 `y` 必须是整数，范围是 -100000 到 100000，否则是 `monitor.invalid`。

启用方式、前端方法和 hello 的 *monitors* 按钮见 [plugins.md](plugins.md)。

## 打印与开发者工具

`webview` 插件对调用窗口做两件事：打开系统打印对话框，以及开关开发者工具。空的窗口 label 按 `main`。

| 命令 | 返回 |
| --- | --- |
| `webview:print` | `null`。CEF `Print()` 弹出系统打印对话框。不写 PDF 文件 |
| `webview:open_dev_tools` | `null`。已经打开时也成功，不会再开一扇 |
| `webview:close_dev_tools` | `null`。已经关闭时也成功 |
| `webview:is_dev_tools_open` | 布尔值 |

开发者工具还要 `plugin.webview.devtools` 为 true。省略时是 false。这时打开、关闭和查询返回 `webview.denied`，不会调用 CEF。打印不看这个开关，用户仍要在系统对话框里确认。

capability 里没有对应命令名时是 `acl.denied`，发生在插件之前。这个 label 没有浏览器，或 CEF 没有在两秒内回答，是 `webview.unavailable`。

`ShowDevTools` 会在 UI 线程上再进入 `OnAfterCreated`。这次创建的浏览器只记在开发者工具集合里，不调用 `WindowBind`，所以不会抢走待绑定的 label，也不会出现在 `EmitAll` 里。来自这扇浏览器的 `cefQuery` 直接失败。最后一扇应用窗口关闭时，宿主先关掉还开着的开发者工具，再退出消息循环。

启用方式、前端方法和 hello 的 *print*、*devtools* 按钮见 [plugins.md](plugins.md)。
