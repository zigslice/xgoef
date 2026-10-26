# 日志与崩溃记录（P3）

插件 `log` 把页面日志追加到当前用户的状态目录，并把宿主 panic 记成最近一次崩溃。页面只能选择级别和一行消息。目录、文件名和崩溃正文都由宿主决定。

Linux 上目录是 `$XDG_STATE_HOME/xgoef`，没有这个变量时是 `~/.local/state/xgoef`。日志文件是 `xgoef-<identifier>.log`，最近一次宿主崩溃是 `xgoef-<identifier>.crash`。`identifier` 里不是字母、数字、`.`、`_`、`-` 的字符会换成 `_`。`XDG_STATE_HOME` 必须是绝对路径，相对路径不会写到当前工作目录。macOS 和 Windows 上不写这两个文件；标准错误仍然可以收日志。文件目标和标准错误都没有时，调用返回 `log.unavailable`。

```json
"plugins": ["log"],
"plugin": {
  "log": {
    "level": "info"
  }
}
```

`level` 可以省略，省略就是 `info`。可选值是 `trace`、`debug`、`info`、`warn`、`error`。`xgoef.conf.json` 在装载时就检查这个字段，即使 `plugins` 里还没有 `log`。低于这个级别的 `log:write` 返回成功，并且不写任何内容。崩溃记录不受这个级别影响。

`log:write` 的参数只有 `level` 和 `message`。`level` 可以省略，省略是 `info`。`message` 不能是空白，不能含有控制字符，最长 4096 字节。多出来的字段是 `log.denied`，这次调用不会写文件。页面不能提交路径，也不能把一条普通日志标成崩溃。

一行的形状是 `2026-10-26T08:11:00.000Z INFO hello`。时间是 UTC。文件以追加方式打开，权限是 `0644`。目录不存在时会创建，权限是 `0755`。目标如果已经是符号链接，返回 `log.denied`，不会跟着链接去写。这个名字上如果是目录，同样是 `log.denied`。文件超过 1 MiB 时，当前文件改名为 `xgoef-<identifier>.log.1`（替换旧的那一份），然后重新开始。备份文件如果是符号链接或目录，这次写入失败，原来的日志保持不动。

宿主在 invoke 的 handler 里 panic 时，页面仍收到 `ipc.panic`。同时宿主把恢复出来的值写成一行 `CRASH`，追加到日志，并用原子替换更新 `.crash` 文件，只保留最近一次。这段正文不是页面传来的参数。

启用 `log` 之后，浏览器进程的命令行会带上绝对路径 `--xgoef-crash-dumps=<状态目录>`。`OnBeforeCommandLineProcessing` 只在浏览器进程（`process_type` 为空）、这个路径是绝对路径、且不含控制字符时，补上尚不存在的 `--crash-dumps-dir` 和 `--enable-crash-reporter`。已经存在的开关不会被改掉。真正写出 crashpad dump 还需要可执行文件旁边的 `chrome_crashpad_handler`。这台环境没有 CEF 头文件，也没有运行 crashpad，所以这里只验证开关策略，没有生成 dump。

窗口的 capabilities 要写上 `log:write`。生成的声明是 `plugins.log.write(level, message)`。`xgoef.js` 另有 `trace`、`debug`、`info`、`warn`、`error`，它们都调用 `log:write`，并且只提交级别和消息。

hello 的 *log* 按钮写一条固定的 info 消息。按下之后写的是当前用户的状态目录。

| 错误码 | 含义 |
| --- | --- |
| `log.invalid` | 级别或消息不合法，或参数不是带消息的对象 |
| `log.denied` | 参数里有级别和消息以外的字段，目标是符号链接或目录，或路径离开了日志目录 |
| `log.unavailable` | 没有可写的文件目标，也没有标准错误 |
| `log.write` | 创建、追加或替换失败 |
