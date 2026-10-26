# xgoef dev（P1）

`xgoef dev` 分期之后拉起发行目录里的宿主，工作目录就是这个发行目录，所以宿主读到的是分期后的 `xgoef.conf.json` 和 `web/`。

宿主看到环境变量 `XGOEF_DEV_SOCK` 时，会连上这条 Unix 套接字。`build.frontendDist`（默认 `web/`）里的文件发生变化后，CLI 把这棵目录重新复制到发行目录的 `web/`，再向套接字写一行 `reload`。宿主发出 `dev-reload`。`xgoef.js` 收到后调用 `location.reload()`。

`xgoef dev` 不把前端签进宿主，也不写 deb / msi / dmg / AppImage。`--assets-only` 不链接。发行目录里还没有宿主文件时，`--assets-only` 只分期并返回。宿主进程退出后，`xgoef dev` 跟着退出。

配置了 `build.beforeDevCommand` 时，拉起宿主之前先用 `sh -c` 跑这一行。`build.devUrl` 是 `http` 或 `https` 时，要等到该地址能连上。宿主退出后这条命令也停。`xgoef dev` 不跑 `beforeBuildCommand`；那一行只在 `xgoef build` 分期前端之前执行。Vite 和 React 模板用这两条命令跑 `npm run dev` 和 `npm run build`，见 [templates.md](templates.md)。

这里没有跑 CEF / LLGo 链接。拉起和热重载用套接字与 `dev-reload` 事件测过。

`xgoef build` 在有 `version` 时写 deb / msi / dmg / AppImage，见 [bundle.md](bundle.md)。页面 command 的超时、取消和错误码见 [ipc.md](ipc.md)。`web/xgoef.d.ts` 见 [binding.md](binding.md)。
