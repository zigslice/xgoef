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
| `OnLifecycle` | `ready`、`exit`、`page-load`，见 [lifecycle.md](lifecycle.md) |

`WindowSpec`: `Label`, `Title`, `URL`, `Width`, `Height`, `Chrome`。`Chrome` 的零值是有装饰、跟随系统、不透明。

## Conf

`app.windows[].label` (default `main`) is applied via `--xgoef-label=` on startup.
Capabilities should list each label that may invoke commands.

窗口菜单和托盘见 [menu.md](menu.md)。第二次启动同一 identifier 时会交给已经在跑的进程，见 [instance.md](instance.md)。

## 记住位置（window-state）

默认会把每个窗口的屏幕坐标、宽高、是否最大化和是否最小化写到应用目录的 `.xgoef/window-state.json`。下次启动按 label 恢复。最大化和最小化都保留恢复前的宽高。从已经最大化的窗口再最小化时，最大化标记留着，取消最小化会回到最大化。记录里有最小化时，下次启动以最小化出现。

`app.windowState` 设为 `false` 就停用。省略这个字段时是开启的。

第一扇窗口的尺寸来自 `--xgoef-size=宽,高`。有保存记录时用记录里的宽高，否则用配置里的 `width` / `height`。再往后 `CreateWindow` 同样优先用保存的宽高；坐标、最大化和最小化由 shim 在创建 Views 窗口时套用。已保存的最小化优先于最大化，已保存的最大化优先于命令行 `--initial-show-state`。

移动、缩放或关闭已经绑定 label 的窗口时，shim 回调宿主写文件。坏掉的 JSON 不会拦住启动，下一次成功记住时整文件重写。

这份状态只走 Views 窗口。`--use-native` 那条路径不读它。

传给 shim 的文本每个窗口一行，六列用制表符分开：`label`、`x`、`y`、`width`、`height`、`flags`。`p` 表示坐标和尺寸都要套用，`m` 表示最大化，`n` 表示最小化。三项都在时写成 `pmn`。`cef/shim/xgoef_window_state.cc` 按同一规则解析，且不依赖 CEF 头文件。

## 当前窗口

`window` 插件作用在发出调用的那一扇 Views 窗口上。参数里不能再写一个 label。空的窗口 label 按 `main`。`--use-native` 没有 Views 窗口，命令返回 `window.unavailable`。把左上角移到另一块屏用 `monitor:move`。

| 命令 | 参数 | 行为 |
| --- | --- | --- |
| `window:minimize` | `{}` | `Minimize` |
| `window:maximize` | `{}` | `Maximize` |
| `window:unminimize` | `{}` | 已经最小化时 `Restore`。没有最小化时成功，状态保持原样 |
| `window:unmaximize` | `{}` | 已经最大化时 `Restore`。没有最大化时成功，状态保持原样 |
| `window:toggle_maximize` | `{}` | 已经最大化则 `Restore`，否则 `Maximize` |
| `window:show` | `{}` | `Show` |
| `window:hide` | `{}` | `Hide` |
| `window:center` | `{}` | 用当前尺寸，在当前显示器上居中 |
| `window:set_title` | `{title}` | `SetTitle`。`title` 必须出现，可以是空字符串。正文是 UTF-8，最长 256 字节，不能含有小于 `0x20` 的字节 |
| `window:set_always_on_top` | `{alwaysOnTop}` | 布尔。`true` 时窗口保持在前面 |
| `window:set_fullscreen` | `{fullscreen}` | 布尔 |
| `window:set_size` | `{width,height}` | `SetSize`。宽和高是 1 到 16384 的整数。位置保持不变 |
| `window:is_minimized` | `{}` | 布尔 |
| `window:is_maximized` | `{}` | 布尔 |
| `window:is_fullscreen` | `{}` | 布尔 |
| `window:is_always_on_top` | `{}` | 布尔 |
| `window:is_visible` | `{}` | 布尔。对窗口来说，真表示用户此刻能看见它 |

多出来的字段、缺了必填字段、或者尺寸不是这个范围内的整数，是 `window.invalid`。这个 label 没有 Views 窗口，或界面线程两秒内没有回答，是 `window.unavailable`。capability 里没有对应命令名时是 `acl.denied`，发生在插件之前。

这些调用在带 CEF 的构建里投到界面线程。动词编号、标题和尺寸规则在 `cef/shim/xgoef_window_op.h`，这份头文件不依赖 CEF。这台环境没有启动 CEF，所以没有看见窗口真的最大化或隐藏。

启用方式和 hello 的 *window* 按钮见 [plugins.md](plugins.md)。

## 缩放和关闭

每个窗口还可以限制能不能缩放、最小化、最大化、关闭，以及最小和最大尺寸。配置写在 `app.windows[]` 上，省略时四项都允许，没有最小或最大尺寸。页面用 `window` 插件改当前这扇 Views 窗口。空的窗口 label 按 `main`。

| 字段 / 命令 | 含义 |
| --- | --- |
| `resizable` / `window:set_resizable` `{resizable}` | 能不能拖边改大小。`window:is_resizable` 返回布尔 |
| `minimizable` / `window:set_minimizable` `{minimizable}` | 能不能最小化 |
| `maximizable` / `window:set_maximizable` `{maximizable}` | 能不能最大化 |
| `closable` / `window:set_closable` `{closable}` | 能不能关闭。关掉之后，标题栏关闭和 `Close` 都会停在委托里，不会先去关浏览器 |
| `minWidth` `minHeight` / `window:set_min_size` `{width,height}` | 最小尺寸，DIP。两边一起写。`0,0` 表示不设下限。`window:min_size` 在没设时返回 `null` |
| `maxWidth` `maxHeight` / `window:set_max_size` `{width,height}` | 最大尺寸。规则和最小尺寸一样。设了的下限不能大于设了的上限 |

第一扇窗口把这八项编成 `--xgoef-limits=可缩放,可最小化,可最大化,可关闭,最小宽,最小高,最大宽,最大高`，例如 `0,1,1,1,320,240,0,0`。再 `CreateWindow` 时，宿主在创建前把同一段文本交给 shim。`cef/shim/xgoef_window_limits.cc` 按同一规则解析，且不依赖 CEF 头文件。Views 委托在每次被问到时读这份表。

宽或高单独出现、超出 1 到 16384、或者下限大于上限，配置加载失败，页面调用是 `window.invalid`。多出来的字段也是 `window.invalid`。`--use-native` 没有 Views 委托，启动开关被忽略，页面命令是 `window.unavailable`。

这台环境没有启动 CEF，所以没有看见窗口真的拒绝缩放或关闭。

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

## 注入脚本

每个窗口可以带 `initScripts`。这是应用配置里的脚本，不是页面提交的。主框架每次加载结束时，如果地址还在这个窗口 URL 的源站上，宿主按顺序执行它们。

源站规则：

- `xgoef://app/...` 只匹配 `xgoef://app`。
- `http` 和 `https` 只匹配相同的协议、主机和端口。省略的端口按 80 或 443。
- 路径、查询和片段不参加比较。
- 带用户名密码、反斜杠、空格的地址拒绝。
- `javascript:`、`data:`、`file:`、`about:` 以及其他协议拒绝。导航到别的源站之后，这些脚本不再注入。

脚本在页面的主世界里执行，页面看得到，也不能靠它藏秘密。`ExecuteJavaScript` 不受页面 CSP 限制，所以正文只能来自配置或宿主。子框架和开发者工具窗口不会注入。一条脚本最长 16 KiB，一个窗口最多 8 条，正文里不能有 NUL。

hello 的主窗口注入 `window.__xgoefInjected = "hello"`。*injected* 按钮读这个值。具名脚本的命令见 [plugins.md](plugins.md)。
