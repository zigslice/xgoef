# 路线图

对标 Tauri 2 的差距与设计见 **[tauri2-gap.md](tauri2-gap.md)**（P0/P1/P2 定义以该文为准）。

| 阶段 | 内容 | 状态 |
| --- | --- | --- |
| 0 | 调研 + 目录骨架 + IPC/配置可单测 | 完成 |
| 1a | LLGo + `cef/shim`（cefsimple_capi）Host；hello 出窗口（无 cgo） | 完成 |
| 1b | **XGo 一等**：`examples/hello/main.xgo` + `xgo build`（LLGo） | 完成 |
| 1c | llcppg 扩大 capi 绑定，收窄 shim | 待做 |
| 2 | `examples/hello` 弹出 CEF 窗口加载本地 `web/` | 完成 |
| 2b | 对标 Tauri 2 差距分析与设计文档 | 完成 |
| 3 | **P0** 接通 `api/js/xgoef.js` ↔ `ipc.Router` + MessageRouter 桥 | 完成 |
| 3b | **P0** Capabilities 强制执行（`acl` + conf） | 完成 |
| 3c | **P1** Event 跨进程 + Window label/`CreateWindow`/`EmitTo` | 完成 |
| 4 | **P1** Plugin 契约 + 首批系统插件（`fs` 范围、`os`） | 完成 |
| 4c | **P2** `shell` / `clipboard` / `dialog` | 完成 |
| 5 | **P1** `xgoef://app/` 资源协议；CLI 与 CEF 链接合成一次构建 | 完成 |
| 5b | **P1** 前端签进宿主二进制（Ed25519） | 完成（系统安装包签名仍后置） |
| 6 | 框架 API（`runtime`/`ipc`/…）迁 `.xgo` | 待做 |
| 6b | **Classfile**：`xclass` + `*_app.xef`/`*_cmd.xef`（见 [classfile-design.md](classfile-design.md)） | 待做 |
| 7 | **P2** Channels（有序宿主 → 页面消息） | 完成 |
| 7b | **P2** 菜单 / 托盘 | 完成 |
| 7c | **P2** window-state 持久化 | 完成 |
| 7d | **P3** 打包更新器（签过名的前端封包） | 完成 |
| 7e | **P3** 单实例 | 完成 |
| 7f | **P3** deep link | 完成 |
| 7g | **P3** 平台安装包（msi / dmg / deb） | 完成 |
| 7h | **P3** AppImage | 完成 |
| 7i | **P1** `xgoef dev` 热重载并拉起进程 | 完成 |
| 7j | **P2** 异步 command 与取消 | 完成 |
| 7k | **P2** TS 类型与 invoke 声明生成 | 完成 |
| 7l | **P2** 结构化错误码 | 完成 |
| 7m | **P3** `process` 插件 | 完成 |
| 7n | **P3** `notification` 插件 | 完成 |
| 7o | **P3** `global-shortcut` 插件 | 完成 |
| 7p | **P3** 窗口主题、装饰与透明 | 完成（窗口化透明仍是 CEF 限制，见 [window.md](window.md)） |
| 7q | **P3** 多显示器与 DPI | 待做 |

验收金样例：行为对齐 `src/cefdemo`（启动、本地页、关闭），API 对齐 Tauri 的 invoke 心智。
