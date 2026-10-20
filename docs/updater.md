# 更新器（P3）

更新器换的是已经签过名的前端封包，不是 msi / dmg / deb，也不做系统安装包签名。装完要重启，当前进程里已经解开的页面不会当场换掉。

## 发布

`xgoef build` 在签前端的同时，如果配置里有 `version`，会在发行目录写下：

- `xgoef.assets`：独立的前端封包
- `update.json`：版本、说明、封包的 sha256，以及这份清单的 Ed25519 签名

签名用的是同一把 `.xgoef/asset.key`。`version` 为空时不写 `update.json`。`xgoef dev` 不写这两份文件。

清单正文是三行文本，前面加固定头 `xgoef-update-v1`：

```text
xgoef-update-v1
<version>
<notes>
<sha256>
```

`version` 是点分的十进制整数，比如 `0.0.2`。`notes` 不能换行。

## 检查和安装

配置：

```json
"plugins": ["updater"],
"plugin": {
  "updater": { "endpoint": "https://updates.example/app/update.json" }
}
```

`endpoint` 也可以是目录，或相对应用目录的路径。远程地址只接受 `https`。公钥用 `build.assetPublicKey`，没有公钥就不能开这个 endpoint。

窗口的 capabilities 要写上 `updater:check` 和 `updater:install`。

| 命令 | 结果 |
| --- | --- |
| `updater:check` | 有更新时返回 `{version, notes, current}`，没有则是 `null` |
| `updater:install` | 校验通过后替换宿主尾部的封包，并写出旁边的 `xgoef.assets`。返回 `{version, restart:true}` |

比当前版本旧或相同的清单不会安装。校验看三件事：清单签名、sha256、封包自身的 Ed25519 尾部。装过的版本记在 `.xgoef/updater-version`，避免同一份更新每次启动都再装一次。

前端：

```js
const offer = await window.__XGOEF__.plugins.updater.check()
if (offer) {
  await window.__XGOEF__.plugins.updater.install()
}
```
