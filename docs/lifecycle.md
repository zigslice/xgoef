# 应用生命周期（P1）

宿主用 `App.OnLifecycle` 接收三类事件。页面不能发送这些事件，也不能改 `page-load` 的地址。

| 事件 | 何时 |
| --- | --- |
| `ready` | 第一扇浏览器已经创建，消息循环还没开始 |
| `exit` | 消息循环已经返回，插件 `Teardown` 还没跑 |
| `page-load` | 某个窗口的主框架加载结束 |

`ready` 和 `exit` 的 `Label`、`URL` 是空的。`page-load` 带上绑定的窗口 label 和主框架地址，地址来自浏览器。

```go
app.OnLifecycle(func(ev runtime.Lifecycle) {
    switch ev.Event {
    case runtime.LifecycleReady:
        // 可以在这里再 CreateWindow
    case runtime.LifecyclePageLoad:
        // ev.Label, ev.URL
    case runtime.LifecycleExit:
        // 插件还在
    }
})
```

回调按注册顺序执行。`nil` 会被忽略。`page-load` 在 CEF 的 UI 线程上调用。

创建第一扇浏览器失败时，不会发出 `ready` 或 `exit`，宿主会 `Shutdown`。`Initialize` 失败时同样还没进入循环，也不发这两条。

`page-load` 只接受主框架，并且这扇浏览器不是开发者工具。子框架不通知。label 和 URL 都不能为空，不能含有控制字符，label 最长 64 字节，URL 最长 4096 字节。超出这些限制的加载不会通知。同一个主框架再次加载会再通知一次。

`OnLoadEnd` 里，符合上面条件且 label 非空时，仍会按原来的规则跑该窗口的注入脚本，然后再决定要不要通知宿主。这台环境没有跑 CEF，所以没有在真实浏览器里收到 `page-load`。过滤规则和回调接线由单独的测试覆盖。

没有对应的页面命令。hello 也不提供按钮。
