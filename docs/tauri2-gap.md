# 对标 Tauri 2：能力差距与设计

调研基准：[Tauri 2](https://v2.tauri.app/)（文档 / Architecture / IPC / ACL / Plugins，2026-10）。  
对照对象：本仓库当前实现（XGo 应用 + LLGo + CEF C API）。

本文回答三件事：

1. **我们已有什么**（相对 Tauri 的心智模型）
2. **还差什么**（按能力域拆开，区分「未做」与「刻意不做」）
3. **怎么补**（分优先级的设计落点，不估日历工期）

相关文档：`DESIGN.md`（分层）、`RESEARCH.md`（选型）、`docs/roadmap.md`（阶段）、cefutil [`docs/07-javascript.md`](https://github.com/fanfeilong/cefutil/blob/master/docs/07-javascript.md)（CEF JS 桥约束）。

---

## 1. 对标边界（先对齐预期）

| 维度 | Tauri 2 | xgoef | 含义 |
| --- | --- | --- | --- |
| 渲染 | 系统 WebView（WRY/TAO） | **自带 Chromium 的 CEF** | 包体更大、行为更齐、跨发行版一致；**不追求 Tauri 的小体积** |
| 后端语言 | Rust（+ 插件侧 Swift/Kotlin） | **XGo 一等**，底层可混编 Go；LLGo 链 CEF | API 形状学 Tauri，实现栈不同 |
| 平台 | Desktop + **Mobile** | **Desktop-first**（现阶段 Linux 可跑） | 移动端标为非目标 |
| 安全模型 | Capabilities / ACL 一等公民 | 命令级默认拒绝；`fs` 另有目录范围 | 远程 URL ACL 与通用 scope 引擎仍缺 |
| 插件生态 | 官方插件矩阵 + 社区 | 仅接口骨架 | 官方插件不照搬，先定契约再挑子集 |

**刻意不对齐的点（非缺口）：**

- 不换成系统 WebView；继续 CEF（本仓库定位）。
- 不追求 &lt;1MB 安装包；接受 `libcef` 量级。
- 不复制 Tauri 移动端与全部官方插件市场。
- 更新器 / 签名分发可后置；本仓库已有 `docs/10-ship-and-upgrade.md` 可复用思路。

---

## 2. 当前 xgoef 快照（「有」到什么程度）

| 能力 | 状态 | 证据 |
| --- | --- | --- |
| 应用用 XGo 书写 | ✅ | `examples/hello/main.xgo`，`xgo build` → LLGo |
| 窗口 + 本地页 | ✅（薄） | CEF Views/`cefsimple_capi` 路径；URL 经 `--url=` |
| 配置文件 | 🟡 | `xgoef.conf.json` + schema；窗口位置写在 `.xgoef/window-state.json` |
| Command 路由（宿主侧） | ✅ | `ipc.Router` 经 MessageRouter 接到前端 |
| Event 总线（宿主侧） | ✅ | `Emit` / `EmitTo` 跨窗口，见 `docs/events.md` |
| 前端 JS 桥 | ✅ | `api/js/xgoef.js` + `window.cefQuery` |
| Plugin 生命周期 | ✅ | 契约 + `fs` / `os` / `shell` / `clipboard` / `dialog`；见 `docs/plugins.md` |
| Capabilities | ✅ | `acl.Guard` 默认拒绝 |
| CLI（create/dev/build） | 🟡 | 生成骨架、分期目录并链接 CEF 宿主；`xgoef dev` 会拉起宿主并热重载（见 `docs/dev.md`）。`xgoef build` 在有 `version` 时写 deb / msi / dmg / AppImage（见 `docs/bundle.md`），并写 `web/xgoef.d.ts`（见 `docs/binding.md`） |
| 多窗口 / 托盘 / 菜单 | 🟡 | label / `CreateWindow`；菜单栏与托盘菜单已接（见 `docs/menu.md`）。常驻图标要能加载 `libgtk-3.so.0` |
| 资源协议 / 资产嵌入 | ✅ | `xgoef://app/`；`xgoef build` 把 `web/` 以 Ed25519 签进宿主 |
| 官方类插件（fs/dialog/…） | 🟡 | `fs`、`os`、`shell`、`clipboard`、`dialog`；notification 等未做 |

结论：前端 invoke、ACL、多窗口事件、资源协议、这批系统插件、`xgoef build` 一次完成分期和链接、签进宿主的前端、Channels、菜单和托盘、window-state、签过名的前端更新、单实例、deep link、deb / msi / dmg / AppImage、`xgoef dev` 的拉起和热重载、带超时和取消的异步 command、invoke 的 TypeScript 声明，以及结构化错误码，都已经接上。

---

## 3. Tauri 2 能力地图 → xgoef 差距矩阵

图例：✅ 已有 · 🟡 部分 · ⬜ 缺失 · ✕ 刻意不做（或远期）

### 3.1 核心运行时与窗口

| Tauri 能力 | xgoef | 差距说明 |
| --- | --- | --- |
| App / Builder 生命周期 | 🟡 | 有 `runtime.App.Run`；缺精细 hook（ready / exit / page-load） |
| Window / Webview 多实例 | 🟡 | label + `CreateWindow` / `CloseWindow` / `FocusWindow`；托盘见 `docs/menu.md` |
| Window 状态（位置/尺寸/最大化） | 🟡 | 位置、尺寸、最大化写入 `.xgoef/window-state.json`（见 `docs/window.md`）；最小化不恢复 |
| 菜单 / 托盘 / 任务栏 | 🟡 | 窗口菜单栏和托盘菜单已封装（`docs/menu.md`）；任务栏未做。常驻图标在 `libgtk-3.so.0` 能加载时出现 |
| 主题 / 装饰 / 透明 | ⬜ | — |
| 多显示器 / DPI | ⬜ | 依赖 CEF 默认 |
| 打印 / 开发者工具 API | ⬜ | CEF 支持，未暴露 XGo API |

### 3.2 IPC（Commands / Events / Channels）

| Tauri 能力 | xgoef | 差距说明 |
| --- | --- | --- |
| `invoke` → 后端 command | ✅ | JS → MessageRouter → `ipc.Router` |
| 命令参数/返回值 JSON | ✅ | TypeScript 声明见 `docs/binding.md`；错误码见 `docs/ipc.md` |
| 异步 command | ✅ | 页面 invoke 在回调外执行；可取消、可超时（见 `docs/ipc.md`） |
| Events 双向 | ✅ | `emit` / `listen` 与 `Emit` / `EmitTo` |
| 按 webview/window 定向 emit | ✅ | `App.EmitTo(label, …)` |
| Channels（有序大数据流） | ✅ | `ctx.Channel` / `__XGOEF__.channel()`，见 `docs/channels.md` |
| `eval` / 注入脚本 | ⬜ | CEF 可做；未封装 |
| IPC 权限门禁 | ⬜ | 见 ACL |

**P0 设计落点（IPC）：** 见 §5.1。

### 3.3 安全：Capabilities / ACL / CSP

| Tauri 能力 | xgoef | 差距说明 |
| --- | --- | --- |
| Capability 绑定窗口/webview | ✅ | `acl.Guard.Allows(label, cmd)` |
| Permission 标识（`plugin:perm`） | 🟡 | 用命令名（`fs:read_text`），没有独立 permission 对象 |
| 路径/命令 scope（allow/deny） | 🟡 | `fs` 有目录 allow/deny；通用 scope 引擎没有 |
| remote URL 能力边界 | 🟡 | 本地导航限制在前端根；`http(s)` 仍放行（devUrl） |
| CSP / 资产完整性 | 🟡 | 嵌入前端有 Ed25519 校验；没有 CSP |
| 隔离：未匹配 capability → 无 IPC | ✅ | 默认拒绝 |

**P0 设计落点（ACL）：** 见 §5.2。

### 3.4 前端工程与资产

| Tauri 能力 | xgoef | 差距说明 |
| --- | --- | --- |
| `@tauri-apps/api` 风格包 | 🟡 | 单文件 `xgoef.js`；无 npm 包、无 TS |
| `withGlobalTauri` / 自动注入 | ⬜ | 需在 `OnContextCreated` 注入桥 |
| `frontendDist` / `devUrl` 热更新 | ✅ | `xgoef dev` 拉起宿主，前端变更后复制并 `location.reload()`（见 `docs/dev.md`） |
| 自定义协议（`asset://` / `tauri://`） | ✅ | `xgoef://app/`，见 `docs/asset.md` |
| 图标 / 资源嵌入打包 | 🟡 | 前端已签进宿主；图标和 CEF Resources 仍随目录拷贝 |

### 3.5 插件系统

| Tauri 能力 | xgoef | 差距说明 |
| --- | --- | --- |
| Plugin = 原生逻辑 + 可选 JS API | 🟡 | `Setup` 注册命令；JS 在 `xgoef.js` 的 `plugins.fs` / `plugins.os` |
| 插件注册 command / 权限声明 | ✅ | `Permissions()` + capabilities 精确匹配 |
| 官方插件矩阵（fs/dialog/shell/…） | 🟡 | `fs`、`os`、`shell`、`clipboard`、`dialog`；notification 等未做 |
| 移动端插件（Swift/Kotlin） | ✕ | 非目标 |

**建议首批官方插件（桌面）：** `fs`（scoped）、`dialog`、`clipboard`、`shell`（受限 open）、`os`、`process`、`notification`、`global-shortcut`。`window-state` 已做在 runtime 里。其余（sql/updater/websocket…）后置。

### 3.6 工具链与 DX

| Tauri 能力 | xgoef | 差距说明 |
| --- | --- | --- |
| `create-tauri-app` | 🟡 | `xgoef create <name>` 写出 XGo 骨架 |
| `tauri dev` / `tauri build` | 🟡 | `xgoef dev` 拉起宿主并热重载（见 `docs/dev.md`）；`xgoef build` 在有 `version` 时写 deb / msi / dmg / AppImage，并写 `web/xgoef.d.ts` |
| 多前端模板（Vite/React/…） | ⬜ | — |
| 平台打包（msi/dmg/deb/AppImage） | ✅ | deb / msi / dmg / AppImage 见 `docs/bundle.md`。系统安装包签名仍后置 |
| IDE / 代码生成（command 宏） | 🟡 | `xgoef build` / `xgoef dev` 写出 `web/xgoef.d.ts`（见 `docs/binding.md`） |

### 3.7 发行与运维

| Tauri 能力 | xgoef | 差距说明 |
| --- | --- | --- |
| Updater 插件 | 🟡 | `updater:check` / `updater:install` 替换签过名的前端封包（见 `docs/updater.md`）；不换系统安装包 |
| 单实例 | ✅ | 同一 identifier 的后续启动聚焦已有进程（见 `docs/instance.md`） |
| Deep link | ✅ | 当前用户的 XDG scheme handler（见 `docs/deeplink.md`） |
| Autostart | ⬜ | — |
| 崩溃报告 / 日志插件 | ⬜ | CEF crashpad 可接 |

### 3.8 移动端

| Tauri 能力 | xgoef |
| --- | --- |
| iOS / Android | ✕ 非目标（CEF 移动路径与 Tauri 不同） |

---

## 4. 优先级（按「产品可用」排序）

### P0 — 没有就不叫 Tauri-like 应用

1. **IPC 传输接通**：`xgoef.js` ↔ `ipc.Router`（CEF MessageRouter 或 process message + C shim）。
2. **桥注入**：浏览器上下文创建时注入 `__XGOEF__` / `__xgoefPost`。
3. **Capabilities 强制执行**：未授权 command → 拒绝；默认拒绝未声明窗口。
4. **hello 端到端**：按钮 `invoke('greet')` 返回宿主字符串（不再 *host bridge not connected*）。

### P1 — 可做正经桌面壳

5. Window API：label、多窗、关闭/聚焦、基础状态。 ✅（见 `docs/window.md`）
6. 自定义资源协议（或严格 `asset` 根目录）替代裸 `file://`。 ✅（见 `docs/asset.md`）
7. Event 跨进程（含 `emit_to(label)`）。 ✅（见 `docs/events.md`）
8. Plugin 契约：注册 commands + `fs` / `os`。 ✅（见 `docs/plugins.md`）
9. `xgoef dev` / `xgoef build` 分期目录并链接 CEF 宿主。 ✅（热重载与拉起见 `docs/dev.md`；`--assets-only` 在没有宿主文件时只分期）

### P2 — 体验对齐

10. Channels、异步 command、超时/取消。 ✅（Channels 见 `docs/channels.md`，异步与取消见 `docs/ipc.md`）
11. 首批系统插件：fs（scoped）、dialog、clipboard、os、shell。 ✅（见 `docs/plugins.md`）
12. 菜单 / 托盘 ✅（见 `docs/menu.md`）；window-state ✅（见 `docs/window.md`）。
13. TS 类型与 `invoke` 声明生成。 ✅（见 `docs/binding.md`）

### P3 — 发行与生态

14. updater ✅（见 `docs/updater.md`）；单实例 ✅（见 `docs/instance.md`）；deep link ✅（见 `docs/deeplink.md`）；平台安装包 ✅（deb / msi / dmg / AppImage，见 `docs/bundle.md`）；`xgoef dev` 拉起并热重载 ✅（见 `docs/dev.md`）；异步 command ✅（见 `docs/ipc.md`）；invoke 的 TypeScript 声明 ✅（见 `docs/binding.md`）；结构化错误码 ✅（见 `docs/ipc.md`）。系统安装包签名仍后置。
15. 更多插件；文档站点与模板。

---

## 5. 关键缺口的设计草案

### 5.1 IPC 传输（P0）

**目标：** 前端 `invoke` 进入 `ipc.Router.Invoke`，响应回 Promise。

**推荐路径（与 cefutil [`docs/07-javascript.md`](https://github.com/fanfeilong/cefutil/blob/master/docs/07-javascript.md) 一致）：**

1. **短期（shim 扩展）**  
   - 在 `cef/shim` 增加 MessageRouter（或简化的 `cef_process_message`）C API。  
   - 浏览器进程：收到 query → 调 `App.Router.Invoke` → 回写。  
   - 渲染进程：`xgoef.js` 的 `__xgoefPost` 接到 `cefQuery` / 等价物。

2. **中期**  
   - llcppg 生成更多 `capi`；shim 只保留回调难写部分。

**消息约定（已与 `xgoef.js` 对齐，保持稳定）：**

```json
{ "type": "invoke", "id": 1, "cmd": "greet", "args": { "name": "xgo" } }
{ "type": "response", "id": 1, "ok": true, "payload": "hello, xgo (from xgoef)" }
{ "type": "response", "id": 1, "ok": false, "error": { "code": "ipc.canceled", "message": "ipc: canceled" } }
{ "type": "event", "event": "tick", "payload": { "n": 1 } }
```

失败响应的 `error` 是 `{ code, message }`。记号和 `ipc.Fail` 见 [ipc.md](ipc.md)。

**线程：** UI/浏览器线程投递；重活丢到工作线程再 `post` 回 UI（CEF 规则不变）。

### 5.2 Capabilities（P0）

**配置（演进现有 schema，对齐 Tauri 语义但简化）：**

```json
"capabilities": [
  {
    "identifier": "main",
    "windows": ["main"],
    "permissions": ["greet", "core:default"]
  }
]
```

**运行时：**

- 每个 WebView 绑定 window label（首版默认 `"main"`）。
- `Invoke` 前：`cap.Allows(label, cmd)`；失败返回明确错误（勿静默）。
- 插件权限形如 `fs:read`；应用命令用短名 `greet`。
- **默认拒绝**：无匹配 capability 的窗口不能 IPC。

一期不做 remote URL ACL、不做路径 scope 引擎；fs 插件落地时再加 allow 列表。

### 5.3 Window 模型（P1）

```text
App
 └─ Window (label, title, bounds, url)
      └─ WebView (CEF browser) 
```

- conf `windows[].label`（缺省 `main`）。
- XGo API 草图：`app.CreateWindow {label, title, width, height, url}`、`app.Window(label).Close()`。
- 映射 CEF Views / Alloy：优先继续 `cefsimple_capi` Views 路径，避免两套窗口栈。

### 5.4 资源与协议（P1）

已落地，见 [asset.md](asset.md)。

- 开发：`http(s)` `devUrl` 原样打开。
- 生产：`frontendDist` 映射为 `xgoef://app/...`。`xgoef build` 把这份目录签进宿主，启动时校验后解包再映射。
- 前端根以外的本地路径拒绝导航。`fs` 插件仍用自己的 allow 列表。公钥配置后，`$RESOURCE` 指向解出来的那份。

### 5.5 Plugin 契约（P1，已落地 fs / os）

见 [plugins.md](plugins.md)。配置 `plugins: ["fs", "os"]` 在启动时装载。JS 命名空间是 `__XGOEF__.plugins.fs`。

- **作者体验（classfile）**：应用侧用 `*_cmd.xef` / 内嵌 `*_plugin.xef`；可发布插件用包契约。详见 [classfile-design.md](classfile-design.md)。

### 5.6 CLI（P1）

| 命令 | 行为 |
| --- | --- |
| `xgoef create` | 模板：`main.xgo` + `xgoef.conf.json` + `web/` + `api/js/xgoef.js` + `xgoef.d.ts` |
| `xgoef dev` | 链接宿主并分期到 `build/dev`，写出 `web/xgoef.d.ts`；若设置了 `devUrl`，改写首个窗口 URL。然后拉起宿主，前端一变就重新复制并让页面重载（见 `docs/dev.md`） |
| `xgoef build` | 链接 shim、拷贝 CEF 运行时、`xgo build` 宿主，并分期 conf / 前端 / JS，再写 `web/xgoef.d.ts`。有 `version` 时再写 `bundle/` 里的 deb、msi、dmg、AppImage。`--assets-only` 不链接，仍会在有 `version` 时写这四份包 |

工具链环境由 `scripts/build-xgoef-hello.sh` 设置后执行 `xgoef build`。安装仍用 `scripts/install-xgo-toolchain.sh`。cmake 与 `xgo build` 只写在 CLI 里。

---

## 6. 架构演进图（目标态）

```text
┌────────────────────────────── Web ──────────────────────────────┐
│  @xgoef/api (TS)  ·  __XGOEF__.invoke / listen / channel         │
└───────────────────────────────┬─────────────────────────────────┘
                                │ CEF MessageRouter / scheme
┌───────────────────────────────▼─────────────────────────────────┐
│  runtime.App (XGo/Go)                                             │
│   ├─ acl.Guard(capabilities)                                      │
│   ├─ ipc.Router + EventHub + Channel                              │
│   ├─ window.Registry                                              │
│   └─ plugin.Registry → fs/dialog/...                              │
└───────────────────────────────┬─────────────────────────────────┘
                                │ LLGo C ABI
┌───────────────────────────────▼─────────────────────────────────┐
│  cef/ (llcppg + thin shim)  →  libcef                             │
└─────────────────────────────────────────────────────────────────┘
```

---

## 7. 验收标准（相对 Tauri 心智，而非功能清单）

**里程碑 A（P0 完成）：**

- hello 页点击 *invoke greet* 显示宿主返回值。
- 未在 capabilities 声明的 command 调用失败。
- 文档：`docs/run.md` 描述桥接与 ACL。

**里程碑 B（P1 核心）：**

- 至少两个 window label；从宿主 `emit` 到指定窗。
- 一个 scoped `fs` 插件可演示（hello 的 *read note*）。 ✅
- `xgoef build` 打出可拷贝的资源目录、宿主二进制，以及签进二进制的前端。 ✅（系统安装包签名仍后置）

**里程碑 C：**

- Channels 落地。 ✅（见 `docs/channels.md`）
- 菜单和托盘落地。 ✅（见 `docs/menu.md`）
- window-state 落地。 ✅（见 `docs/window.md`）
- 签过名的前端更新落地。 ✅（见 `docs/updater.md`）
- 单实例落地。 ✅（见 `docs/instance.md`）
- deep link 落地。 ✅（见 `docs/deeplink.md`）
- 平台安装包落地。 ✅（deb / msi / dmg / AppImage，见 `docs/bundle.md`）。系统安装包签名仍后置。
- `xgoef dev` 拉起进程并热重载。 ✅（见 `docs/dev.md`）
- 异步 command、超时与取消。 ✅（见 `docs/ipc.md`）
- invoke 的 TypeScript 声明。 ✅（见 `docs/binding.md`）
- 结构化错误码。 ✅（见 `docs/ipc.md`）

---

## 8. 风险与依赖

| 风险 | 影响 | 缓解 |
| --- | --- | --- |
| MessageRouter C API 绑定成本 | 卡住 P0 | 先 shim 手写最小 query/response |
| CEF 多进程 vs LLGo 宿主 | 辅助进程不稳 | 保持 `xgoef-helper`；GPU 默认 disable |
| ACL 过严/过松 | 安全或 DX | 默认拒绝 + hello 明示 permission |
| 插件范围膨胀 | 失焦 | 严格按 §3.5 首批列表 |
| 与 Tauri API 名一一对应强迫症 | 无意义工时 | **对齐心智，不保证符号同名** |

---

## 9. 文档与代码后续动作

1. 实现 P0：shim 桥 + ACL + hello E2E（更新 `roadmap.md` 状态）。
2. 扩展 `xgoef.conf.schema.json`：`windows[].label`、正式 `capabilities[]`。
3. 新增 `docs/ipc.md`、`docs/acl.md`（实现时写；本稿为总设计）。
4. `DESIGN.md` 仅保留分层总览，差距与演进以本文为准。

---

## 10. 一句话结论

相对 Tauri 2，xgoef 已接通 **invoke、ACL、多窗口事件、`fs`/`os`/`shell`/`clipboard`/`dialog`、`xgoef://app/`、`xgoef build` 一次完成资源分期和 CEF/LLGo 链接、Ed25519 签进宿主的前端、Channels、菜单和托盘、window-state、签过名的前端更新、单实例、deep link、deb / msi / dmg / AppImage、`xgoef dev` 的拉起和热重载、带超时和取消的异步 command、invoke 的 TypeScript 声明，以及结构化错误码**。接下来的产品差距集中在：

1. **`process` 插件**（首批里还剩 `notification`、`global-shortcut`）

刻意保留 CEF 与桌面范围，不在体积和移动端上与 Tauri 竞赛。
