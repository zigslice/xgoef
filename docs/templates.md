# 前端模板

`xgoef create <name>` 默认写出静态页，模板名是 `html`。另外四套模板把页面交给 Vite，生产构建仍落到 `web/`，由 `xgoef://app/` 提供。

```bash
xgoef create myapp
xgoef create myapp --template vite
xgoef create myapp --template react
xgoef create myapp --template vue
xgoef create myapp --template svelte
```

`--template` 可以写在名字前面。可选值是 `html`、`vite`、`react`、`vue`、`svelte`。

| 模板 | 页面 | 开发时窗口地址 | 构建产物 |
| --- | --- | --- | --- |
| `html` | `web/index.html` | `xgoef://app/index.html` | 就是这份 `web/` |
| `vite` | Vite 的 `index.html` 和 `src/main.js` | `http://127.0.0.1:5173` | `web/` |
| `react` | Vite + React，入口是 `src/main.jsx` | `http://127.0.0.1:5173` | `web/` |
| `vue` | Vite + Vue，入口是 `src/main.js`，页面是 `src/App.vue` | `http://127.0.0.1:5173` | `web/` |
| `svelte` | Vite + Svelte，入口是 `src/main.js`，页面是 `src/App.svelte` | `http://127.0.0.1:5173` | `web/` |

`vite`、`react`、`vue` 和 `svelte` 的 `build.devUrl` 是 `http://127.0.0.1:5173`。`beforeDevCommand` 是 `npm run dev`，`beforeBuildCommand` 是 `npm run build`。Vite 把 `base` 设为 `./`，`outDir` 设为 `web`，这样打包后的资源地址在 `xgoef://app/` 下仍能对上。端口写死在 `vite.config.js` 和 `devUrl` 里，要改就两处一起改。

`xgoef dev` 先跑 `beforeDevCommand`。`devUrl` 是 `http` 或 `https` 时，等到这个地址能连上再拉起宿主，然后把第一个窗口的 URL 改成它。宿主退出或收到中断信号时，这条命令也会停。`xgoef dev` 不跑 `beforeBuildCommand`。

`xgoef build` 在分期前端之前跑 `beforeBuildCommand`。命令非 0 退出时，这次构建失败，不会去链接宿主。`--assets-only` 同样会先跑它。

两条命令都是配置里的一行 shell，由 `sh -c` 在应用目录执行。空字符串就跳过。最长 1024 字节，不能含有换行或其它控制字符。这是应用自己的配置，不是页面传来的字符串。

`create` 不执行 `npm install`。Vite 模板的下一步是：

```bash
cd myapp
npm install
xgoef dev --app .
```

发行构建是 `npm` 已经装好依赖之后的 `xgoef build`。`web/` 里先放了一份占位页，第一次 `vite build` 会把它换掉。`public/note.txt` 会进 `web/note.txt`，页面上的 *read note* 读的就是它。

渲染进程仍在上下文创建时注入 `xgoef.js`。模板页面不写 `<script src="xgoef.js">`。开发服务器上的页面和打包进 `xgoef://app/` 的页面都走 `__XGOEF__.invoke`。
