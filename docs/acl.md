# Capabilities / ACL（P0）

对齐 Tauri 心智：默认拒绝；窗口 label 必须匹配 capability。

## 配置

```json
"capabilities": [
  {
    "identifier": "main",
    "windows": ["main"],
    "permissions": ["greet", "core:default"]
  }
]
```

- `windows`：空表示所有窗口；`*` 通配。
- `permissions`：与 command 名精确匹配（如 `greet`）。`core:default` 本身不放行任意命令。
- 加载器仍接受旧版 `{ "main": { "commands": ["greet"] } }` 并转换为上表。

## 运行时

`runtime.App` 在每次 invoke 前调用 `acl.Guard.Allows(windowLabel, cmd)`。拒绝时返回：

```text
acl: command "…" not permitted for window "…"
```

命令权限仍是精确匹配。`fs` 的路径范围不在这里做，见 [plugins.md](plugins.md)。
