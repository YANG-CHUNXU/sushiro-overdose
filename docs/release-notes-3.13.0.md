# v3.13.0

## feat: 桌面端 MCP 助手一键开关

承接 v3.12.0 的 MCP server，本次加**桌面端设置页集成**：用户在设置页点开关就能用 MCP，不用手动配 claude_desktop_config.json。

### 设计（关键修正）

Claude Desktop 自己通过 `claude_desktop_config.json` 拉起 MCP server（command/args/env）。
桌面端**不常驻 MCP 子进程**（避免和 Claude Desktop 起的重复、避免孤儿进程）。
桌面端职责：
1. **准备 venv**（首次启用，让 Claude Desktop 能拉起 Python MCP）
2. **写/删 claude_desktop_config.json 的 sushiro 条目**（开关，合并写不覆盖用户其他 MCP）
3. **开机自启**（确保 venv 就绪，无需常驻进程）

### 改动

- **设置页"MCP 助手"折叠卡**（`web_static.go`）：开关 + 开机自启开关 + 数据库地址/只读密钥输入 + 状态显示（Python 就绪/已注册 Claude Desktop/数据库已配）
- **`mcp_config.go`**：MCPConfig{Enabled,AutoStart,TursoURL,TursoToken}，存 `~/.sushiro/mcp_config.json`（0600）
- **`mcp_manager.go`**：venv 自动准备（uv 优先，回退 python3）+ claude_desktop_config.json 合并写/删 + 状态查询
- **`web_mcp.go`**：`/api/mcp`（GET 状态 / POST 改配置+启停）、`/api/mcp/autostart`
- **平台自启**（`platform_{darwin,linux,windows}.go`）：照搬采样器自启模式，`--mcp-daemon-child` 子命令（确保 venv 就绪后退出，不常驻）
  - macOS：LaunchAgent plist
  - Linux：systemd user unit（oneshot）
  - Windows：注册表 Run 项

### 用户体验流程

1. 设置页 → "MCP 助手" → 填数据库地址 + 只读密钥（去 turso.tech 建只读 token）
2. 开"启用 MCP 助手" → 桌面端自动建 venv + 装依赖（首次几十秒）+ 注册到 Claude Desktop
3. 重启 Claude Desktop → AI 能调 sushiro 的 12 个 tool
4. 可选开"开机自启"

### 安全

- 写接口走 CSRF + Origin 校验（跟其他写接口一致）
- 只读密钥存 0600，不回显到前端
- UI 不暴露 "Turso" 技术词（用"数据库地址/只读密钥"，保持 v3.x 的产品决策）
- MCP server 全只读（v3.12.0 已定）

### 前置

- 本机需 Python ≥ 3.9（首次启用自动建 venv；无 Python 会提示安装）
- Claude Desktop（注册依赖它；没装会提示，venv 仍准备好）

> 仅改桌面端 Go（设置页 + 平台 + handler），MCP server 本体（mcp/）是 v3.12.0 已发的 Python。
