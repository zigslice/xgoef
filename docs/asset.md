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

`xgoef://app/` 的根就是 `web/`。`xgoef.js` 必须放在这个目录里，页面写：

```html
<script src="xgoef.js"></script>
```

`../api/js/xgoef.js` 会走出根，scheme 不会读。`xgoef build` 和 `scripts/build-xgoef-hello.sh` 会把桥复制到 `web/xgoef.js`。

## 代码

| 位置 | 作用 |
| --- | --- |
| `asset.Root` | Go 侧映射和拒绝规则（单测） |
| `runtime.App.NavigationURL` | 启动 URL / `CreateWindow` |
| `cef/shim/xgoef_asset.cc` | CEF `CefSchemeHandlerFactory` |
| `cef/shim/xgoef_asset_path.cc` | 与 Go 对齐的路径规则（不链接 CEF，可单独编译） |
