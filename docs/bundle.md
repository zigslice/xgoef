# 平台安装包（P3）

`xgoef build` 在配置里有 `version` 时，把分期目录打进发行目录的 `bundle/`：

- `<包名>_<版本>_<架构>.deb`
- `<包名>_<版本>_<架构>.msi`
- `<包名>_<版本>.dmg`

`version` 为空时跳过，和 `update.json` 一样。`xgoef dev` 不写这三份文件。`xgoef build --assets-only` 不链接宿主，只要 `version` 非空，仍会把已经分期的文件打进这三份包。

三份包装的是同一棵分期树。本仓库编出的宿主是 Linux ELF。deb 是能在当前 Linux 上安装的那一份：文件进 `/usr/lib/<包名>/`，产品二进制存在时 `/usr/bin/<包名>` 链到它，并写入 `/usr/share/applications/<包名>.desktop`。登记了 scheme 时，这份 desktop 带 `x-scheme-handler/<scheme>`，`Exec` 带 `%u`。

msi 把同一棵树放到 `Program Files/<产品名>/`。dmg 是未压缩的 UDIF，里面是 FAT16 的 `<产品名>.app`：`Contents/MacOS/` 放产品二进制的副本，`Contents/Resources/` 放分期文件，`Info.plist` 在有 scheme 时写 `CFBundleURLTypes`。这两份在本环境里做格式检查（`msiextract`、磁盘镜像读取）。Windows 与 macOS 上要安装的可执行文件，需要在对应系统上链接；包里这份 ELF 只随分期树一起被包进去。

包不做 Authenticode、codesign 或其它系统安装包签名。更新器仍然只换签过名的前端封包，见 [updater.md](updater.md)。

版本要能同时当 Debian 版本（以数字开头，字符集 `[0-9A-Za-z.+~-]`）和 MSI 版本（最多三段整数，不足的段补 `0`）。产品名收成 Debian 包名：小写，其余字符换成 `-`。架构按 `GOARCH`：`amd64`、`i386`、`arm64`、`armhf`。

AppImage 仍后置。
