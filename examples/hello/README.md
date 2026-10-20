# hello

最小 **XGo** xgoef 应用：`main.xgo` + 配置 + `greet` 命令 + 本地 `web/`。

页上还可以 *read note*（`fs:read_text`，读 `web/note.txt`）和 *os platform*（`os:platform`）。未授权命令会被 ACL 拒绝。说明见 [docs/plugins.md](../../docs/plugins.md)。

```bash
# 工具链（一次）
./scripts/install-xgo-toolchain.sh

# 构建并运行（XGo → LLGo，无 cgo）
./scripts/build-xgoef-hello.sh
cd build/hello && ./xgoef-hello
```

应用入口是 `main.xgo`（不是 `.go`）。框架底层的 `cef` / `runtime` 等包仍可为 Go，与 XGo 同模块混编；新产品代码默认写 XGo。

无 CEF 时可用 gc 测框架包：

```bash
go test ./...
```
