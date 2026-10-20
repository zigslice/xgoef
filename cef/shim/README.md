# cef/shim

CEF C++ host pad（MessageRouter + Views），供 **LLGo**（`go:linkname`，无 cgo）链接。

| 文件 | 作用 |
| --- | --- |
| `xgoef_host.h` / `xgoef_host.cc` | lifecycle C ABI：`xgoef_cef_*` |
| `xgoef_ipc.*` | `cefQuery` → C 回调 → `xgoef_ipc_respond` |
| `xgoef_app.*` / `xgoef_handler.*` / `xgoef_renderer.*` | 浏览器 + 渲染进程 MessageRouter |
| `xgoef_asset.cc` / `xgoef_asset_path.cc` | `xgoef://app/` scheme；路径规则与 `asset` 包一致 |
| `xgoef_helper_main.cc` | 原生 `xgoef-helper`（渲染侧注入 `window.cefQuery`） |
| `xgoef_window_state.*` | 窗口位置扁平文本（无 CEF 头）；Views 窗口按它恢复并在移动时回调 |
| `xgoef_menu_model.*` | 菜单扁平文本解析（无 CEF 头） |
| `xgoef_menu.*` | CEF 菜单栏与 `ShowMenu` |
| `xgoef_tray_gtk.cc` | `dlopen("libgtk-3.so.0")` 状态图标；加载失败时仍可用 `tray:popup` |

构建入口：`xgoef build`（`scripts/build-xgoef-hello.sh` 只设置工具链后调用它）。编出 `libxgoef_cef.a` + `xgoef-helper`，再 `xgo build`。

原则：shim **不写业务**；业务在 XGo/Go 的 `runtime` / `ipc` / `acl`。llcppg 扩大 capi 后，此目录应收窄为仅保留回调/Views/MessageRouter 难点。
