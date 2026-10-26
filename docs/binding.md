# TypeScript 声明（P2）

`xgoef build` 和 `xgoef dev` 根据 `xgoef.conf.json` 写出 `web/xgoef.d.ts`，同一份也放在 `api/js/xgoef.d.ts`。`xgoef create` 在新项目里写一份。热重载重新复制前端之后会再写一次。

文件声明 `window.__XGOEF__`。渲染进程会在页面脚本之前装上这个全局，见 [ipc.md](ipc.md)。`invoke` 的命令名、参数和返回值来自配置里的 `commands`，加上已启用插件的内置命令。`menu:popup`、`tray:popup`、`tray:click` 总会出现，因为宿主会注册它们。配置了 `app.deepLink.schemes` 时带上 `deeplink:get_current`。`plugins` 里只列出 `xgoef.js` 已经有的方法，并且只保留这次启用的插件。

```json
"commands": [
  {
    "name": "greet",
    "doc": "返回 hello 文本",
    "args": [{ "name": "name", "type": "string", "optional": true }],
    "result": "string"
  }
]
```

`result` 省略时是 `null`。类型只用这些写法：`string`、`number`、`boolean`、`null`、`unknown`、`XgoefChannel`、`T[]`、`A | B`，以及 `{ name: T; name?: T }`。`XgoefChannel` 就是 `__XGOEF__.channel()` 的返回值，hello 的 `progress` 用它。

生成出来的调用带 `cancel()`，选项里有 `timeout` 和 `signal`，见 [ipc.md](ipc.md)。

`shell.open` 在声明里是一个 `target` 字符串，和 `xgoef.js` 一致。`dialog.open` / `dialog.save` 是一个可选的 `opts` 对象。

失败的调用 reject `XgoefError`。声明里有这个接口，以及 `__XGOEF__.XgoefError`。`code` 是稳定记号，见 [ipc.md](ipc.md)。
