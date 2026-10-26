# 开机自启（P3）

插件 `autostart` 为当前用户写入一条 XDG autostart 桌面项。进程启动时不会自己打开它。页面调用 `autostart:enable` 之后，桌面会话下次登录才会拉起这个应用。

Linux 上文件在 `$XDG_CONFIG_HOME/autostart`，没有这个变量时在 `~/.config/autostart`。文件名是 `xgoef-<identifier>.desktop`。`identifier` 里不是字母、数字、`.`、`_`、`-` 的字符会换成 `_`。`XDG_CONFIG_HOME` 必须是绝对路径。macOS 和 Windows 上这三条命令都返回 `autostart.unavailable`。

```json
"plugins": ["autostart"],
"plugin": {
  "autostart": {
    "args": ["--minimized"]
  }
}
```

`args` 可以省略。最多 8 个，每个 1 到 128 字节，不能是空白，也不能含有控制字符。`xgoef.conf.json` 在装载时就检查这组参数，即使 `plugins` 里还没有 `autostart`。

`Exec` 是宿主的绝对路径，后面接配置里的参数。每一段都加引号。页面不能改可执行文件，也不能改参数；多出来的字段是 `autostart.invalid`。空对象、省略的参数和 `null` 都可以。

`autostart:enable` 创建目录并写入桌面项。目标位置如果已经是符号链接，调用返回 `autostart.denied`，不会跟着链接去写。普通文件会整份换掉，再调用一次仍然成功。`autostart:disable` 只删除这个文件名；符号链接删的是链接本身。文件本来就不在时也算成功。这个名字上如果是目录，返回 `autostart.denied`。

`autostart:is_enabled` 在文件不存在时返回 `false`。文件在，就看第一段 `[Desktop Entry]`：`Hidden=true` 或 `X-GNOME-Autostart-enabled=false` 返回 `false`，其余情况返回 `true`。读入上限是 64 KiB。

应用还要有 `identifier` 和非空的 `productName`。三条命令都要写进窗口的 capabilities：`autostart:enable`、`autostart:disable`、`autostart:is_enabled`。前端是 `plugins.autostart.enable()`、`disable()`、`isEnabled()`。

hello 的 *autostart* 按钮会查询当前状态，再在打开和关闭之间切换。按下之后写的是当前用户的 autostart 目录。

| 错误码 | 含义 |
| --- | --- |
| `autostart.invalid` | 参数不是空对象，启动参数不合法，或可执行文件不是绝对路径 |
| `autostart.unavailable` | 不是 Linux，或 autostart 目录不是绝对路径 |
| `autostart.denied` | 目标是符号链接、目录，或路径离开了 autostart 目录 |
| `autostart.write` | 创建、读写或删除失败 |
