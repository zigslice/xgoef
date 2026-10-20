# Deep link（P3）

`app.deepLink.schemes` 列出这个应用接受的自定义 URL scheme。列表为空，或者省略 `deepLink`，就不注册、也不接收。写了 scheme 就必须有 `identifier`。`http`、`https`、`file`、`xgoef` 这类已经有主人的 scheme 不能占用。

启动时，宿主把当前用户的 XDG 应用目录写成一份 `.desktop`，并在 `mimeapps.list` 里把 `x-scheme-handler/<scheme>` 指到它。桌面文件的 `Exec` 是宿主绝对路径，带 `%u`。`xgoef build` 还会在发行目录放一份同名 `.desktop`，`Exec` 指向该目录里将要生成的宿主文件名。

本次启动参数里匹配的 URL，以及后来同一个 identifier 交过来的参数里匹配的 URL，都会变成事件：

```json
{ "type": "event", "event": "deep-link", "payload": { "urls": ["hello://item/1"] } }
```

页面用 `__XGOEF__.listen("deep-link", handler)` 接收。已经打开的页面也可以调用 `deeplink:get_current`，返回最近一次收到的 `{urls: [...]}`。这个命令要写进窗口的 capabilities。前端封装是 `__XGOEF__.plugins.deeplink.getCurrent()`。

启动时写的是当前用户的 XDG scheme 关联。系统 deb 里的 `.desktop` 和 dmg 的 `Info.plist` 会带上同一组 scheme。Windows 注册表仍后置。见 [bundle.md](bundle.md)。
