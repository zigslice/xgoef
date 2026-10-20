# hello

最小 **XGo** xgoef 应用：`main.xgo` + 配置 + `greet` 命令。页面地址是 `xgoef://app/index.html`（只读 `web/`）。

页上还可以 *read note*（`fs:read_text`）、*os platform* 和 *clipboard*（进程内剪贴板）。未授权命令会被 ACL 拒绝。说明见 [docs/plugins.md](../../docs/plugins.md)。

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
