# Channels（P2）

Channel 是一条有序的宿主 → 页面消息流，用来传进度这类比单个 invoke 返回值更碎的数据。它走和事件相同的 `EmitTo`，但每条消息带递增的 `index`，并且只发给打开它的那个窗口。

## 前端

```js
const ch = window.__XGOEF__.channel()
ch.onmessage((payload) => { /* 按 index 排好序后才调用 */ })
const done = await window.__XGOEF__.invoke("progress", { channel: ch })
ch.close()
```

`channel()` 在参数里会变成 `{"__channel": <id>}`。一次 invoke 只带一个 channel。页面按 `index` 从 0 开始交付；中间缺号会先暂存，凑齐再按顺序调用 `onmessage`。

## 宿主

命令已经过 ACL。Channel 本身不再单独要一条 permission。

```xgo
app.Handle "progress", (ctx, argsJSON) => {
    ch, err := ctx.Channel(argsJSON)
    if err != nil {
        return nil, err
    }
    err = ch.Send(1)
    if err != nil {
        return nil, err
    }
    return "ok", nil
}
```

同一个窗口里的同一个 id 重复打开会拿到原来的 channel，`index` 接着涨。别的窗口可以用同一个数字 id，两条流互不影响。`close` 之后再 `Send` 会失败。

## 信封

```json
{ "type": "channel", "channel": 4, "index": 0, "payload": 1 }
{ "type": "channel-close", "channel": 4 }
```

hello 页的 *channel* 按钮调用 `progress`，依次收到 `1`、`2`、`3`。
