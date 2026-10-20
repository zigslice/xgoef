# 单实例（P3）

同一个 `identifier` 同时只跑一个进程。配置里省略 `app.singleInstance` 时，只要写了 `identifier` 就会打开。设成 `false` 就允许多开。显式写成 `true` 但没有 `identifier` 时，启动失败。

锁是用户运行时目录里的 unix socket（`XDG_RUNTIME_DIR`，没有就用临时目录），文件名来自 identifier。第二个进程把本次的 `os.Args` 交给第一个，然后以退出码 0 结束。第一个进程聚焦第一个窗口，并向页面发事件：

```json
{ "type": "event", "event": "single-instance", "payload": { "args": ["..."] } }
```

页面用 `__XGOEF__.listen("single-instance", handler)` 接收。socket 权限是 `0600`。进程退出后文件会删掉；留下的坏文件下次启动会换掉。这不注册 URL scheme，深度链接仍后置。
