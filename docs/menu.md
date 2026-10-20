# 菜单与托盘（P2）

窗口菜单和托盘菜单都写在 `xgoef.conf.json` 的 `app` 里。点某一项会在当前窗口执行它的 `command`。命令仍走 capabilities：没授权就拒绝，和页面 `invoke` 同一条 ACL。

## 配置

```json
"app": {
  "menu": [
    {
      "id": "file",
      "text": "File",
      "items": [
        {"id": "greet", "text": "Greet", "command": "greet"},
        {"separator": true},
        {"id": "quit", "text": "Quit", "command": "app:quit", "enabled": false}
      ]
    }
  ],
  "tray": {
    "tooltip": "xgoef hello",
    "icon": "",
    "menu": [
      {"id": "greet", "text": "Greet", "command": "greet"}
    ]
  }
}
```

`id` 在同一棵树里唯一。分隔行可以不写 `id`，运行时会补成 `sep-<父级>-<序号>`。`enabled` 省略时该项可用。子菜单可以没有 `command`，这时点它只展开，不执行命令。应用菜单和托盘菜单是两棵树，两边可以用同一个 `id`。

需要弹出菜单的窗口还要声明这三条命令：

| 命令 | 作用 |
| --- | --- |
| `menu:popup` | 在该窗口用 CEF `ShowMenu` 弹出应用菜单 |
| `tray:popup` | 在该窗口用 CEF `ShowMenu` 弹出托盘菜单 |
| `tray:click` | 当作点了一次托盘图标，只发 `tray` 事件 |

## 原生界面

配置了 `app.menu` 时，窗口顶部是一排 CEF `CefMenuButton`。没有菜单时，窗口层次和以前一样，浏览器视图直接铺满。

托盘在能 `dlopen` 到 `libgtk-3.so.0` 时，另起线程放一个 GTK3 status icon。左键发 `tray` 事件，载荷是 `{id:""}`。右键用同一份扁平菜单建 GTK 菜单。GTK 没加载成功时，图标不出现，页面仍可以调用 `tray:popup` 走 CEF 菜单。图标、提示和菜单内容在线程启动时取一次。

shim 不链接 GTK。`pkg-config` 的库列表里也不加 GTK。

## 点击之后

原生点击和 `ClickMenu` / `ClickTray` 走同一条路：查出该项的 `command`，ACL 允许后 `InvokeFrom`，再 `EmitTo`。

```json
{ "type": "event", "event": "menu", "payload": { "id": "greet", "command": "greet" } }
{ "type": "event", "event": "tray", "payload": { "id": "greet", "command": "greet" } }
{ "type": "event", "event": "tray", "payload": { "id": "" } }
```

菜单项事件的 `event` 是 `menu`，托盘是 `tray`。空 `id` 只表示点了图标，不跑命令。

## 传给 shim 的文本

`menu.Menu.Encode` 每个一行，五列用制表符分开：`id`、`text`、`command`、`flags`、`parent`。`flags` 里 `s` 是分隔行，`d` 是禁用。分隔行两项都有，写成 `sd`。根项的 `parent` 为空。父项写在子项前面。`cef/shim/xgoef_menu_model.cc` 按同一规则解析，且不依赖 CEF 头文件。

hello 页的 *menu* / *tray* 按钮分别调用 `menu:popup` 和 `tray:popup`，并监听 `menu` 与 `tray` 事件。
