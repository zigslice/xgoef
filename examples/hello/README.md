# hello

最小 **XGo** xgoef 应用：`main.xgo` + 配置 + `greet` 命令。页面地址是 `xgoef://app/index.html`（只读 `web/`）。

页上还可以 *read note*、*os platform*、*clipboard*、*channel*，以及 *menu* / *tray*。菜单项里的命令仍走 capabilities。窗口的位置和大小会记在应用目录的 `.xgoef/window-state.json`。配置里的 identifier 使第二次启动交给已经打开的进程。`xgoef-hello://` 会交给这个进程。配置里有 `version` 时，`xgoef build` 在发行目录的 `bundle/` 写下 deb、msi 和 dmg。`xgoef dev` 会拉起宿主，并在 `web/` 变化后刷新页面。说明见 [docs/plugins.md](../../docs/plugins.md)、[docs/channels.md](../../docs/channels.md)、[docs/menu.md](../../docs/menu.md)、[docs/window.md](../../docs/window.md)、[docs/instance.md](../../docs/instance.md)、[docs/deeplink.md](../../docs/deeplink.md)、[docs/bundle.md](../../docs/bundle.md) 和 [docs/dev.md](../../docs/dev.md)。

```bash
# 工具链（一次）
./scripts/install-xgo-toolchain.sh

# 构建并运行（脚本设置工具链后执行 xgoef build）
./scripts/build-xgoef-hello.sh
cd build/hello && ./xgoef-hello
```

应用入口是 `main.xgo`（不是 `.go`）。框架底层的 `cef` / `runtime` 等包仍可为 Go，与 XGo 同模块混编；新产品代码默认写 XGo。

无 CEF 时可用 gc 测框架包：

```bash
go test ./...
```
