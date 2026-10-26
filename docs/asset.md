# 资源协议（P1）

打包后的前端不再用裸 `file://` 打开。页面地址是：

```text
xgoef://app/index.html
xgoef://app/css/app.css
```

`app` 这一个 host 对应 `build.frontendDist`（默认 `web/`）。查询字符串和 `#` 不参与找文件。目录地址会去读 `index.html`。

## 范围

只读这个目录里面的文件。

- `..` 清掉之后仍然留在根里；根外的路径不会被映射进来。
- 指到根外的符号链接直接拒绝。
- `file://` 或相对路径如果落在这个目录里，启动时改写成 `xgoef://app/...`。
- 根以外的本地路径拒绝导航。
- `http://` / `https://` 原样保留，给 `devUrl` 用。

宿主在启动参数里带上 `--xgoef-asset-root=<绝对路径>`。CEF 浏览器进程用它注册 scheme handler；`xgoef-helper` 只登记同名 scheme，保证渲染进程认这个 origin。

## 前端桥

`xgoef://app/` 的根就是 `web/`。渲染进程在上下文创建时读取这个根目录里的 `xgoef.js`，装上 `window.__XGOEF__`。页面不必再写：

```html
<script src="xgoef.js"></script>
```

写了也无妨：脚本发现全局已经存在就返回。`../api/js/xgoef.js` 会走出根，scheme 不会读，渲染进程也不从那里读。`xgoef build` 和 `xgoef dev` 会把桥复制到前端根的 `xgoef.js`。关掉自动注入见 [ipc.md](ipc.md)。

## 嵌入与签名

`xgoef build` 在分期完成之后把 `web/` 打成 zip，用 Ed25519 签名，追加到宿主二进制末尾，并写出同样内容的 `xgoef.assets`。公钥写入发行目录里的 `xgoef.conf.json`（`build.assetPublicKey`）。私钥放在应用目录 `.xgoef/asset.key`（已 gitignore），也可以用环境变量 `XGOEF_ASSET_KEY` 指向已有的 64 字节私钥。

配置里带了公钥时，启动会校验宿主尾部；尾部没有封包时再校验旁边的 `xgoef.assets`。通过之后解到 `.xgoef/unpacked`，`xgoef://app/` 和 `$RESOURCE` 都读这个目录。签名不匹配，或公钥已配置但找不到封包，启动直接失败。

`xgoef dev` 和 `--assets-only` 只留下 loose 的 `frontendDist`，方便改页面。`xgoef dev` 还会在这些文件变化时重新复制，并让页面重载，见 [dev.md](dev.md)。配置里有 `version` 时，`xgoef build` 还会在发行目录写 `update.json`，给更新器当 endpoint。系统安装包签名仍后置。见 [updater.md](updater.md)。

## 代码

| 位置 | 作用 |
| --- | --- |
| `asset.Root` | Go 侧映射和拒绝规则（单测） |
| `asset.PackDir` / `AppendSeal` | 把前端签进宿主尾部；`xgoef.assets` 是同一份封包 |
| `runtime.App.NavigationURL` | 启动 URL / `CreateWindow` |
| `cef/shim/xgoef_asset.cc` | CEF `CefSchemeHandlerFactory` |
| `cef/shim/xgoef_asset_path.cc` | 与 Go 对齐的路径规则（不链接 CEF，可单独编译） |
