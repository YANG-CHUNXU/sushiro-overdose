# v3.12.0

## feat: MCP server —— 让 AI 教用户用、查排队数据、给到店建议、联动桌面端

新增独立 MCP server（`mcp/` 子目录），装到 Claude Desktop / Cursor 后，AI 能辅助用户用 sushiro。

### 它做什么

- **查排队数据**：读 Turso 新库的全国历史叫号/压力/速度（"太阳宫凯德几点最挤""叫号快不快"）
- **联动桌面端**：调本地只读 API 查"我的预约/凭证是否有效/排队号状态/环境诊断"
- **智能到店建议**：综合实时 ETA + 历史规律（"想周六晚 6 点去该几点取号"）
- **教用户用**：FAQ resource + explain_usage tool（"怎么拿通行证""凭证失效怎么办"）

### 12 个工具（全只读，标 readOnlyHint）

查数据（读 Turso）：`list_stores` / `store_queue_history` / `store_pressure` / `called_speed` / `compare_stores`
联动桌面端（调 39871 GET）：`desktop_status` / `my_reservations` / `my_ticket_status` / `diagnose` / `available_slots`
综合：`arrival_advice` / `explain_usage`
Resource：`docs://faq`

### 技术

- Python + 官方 `mcp` SDK（FastMCP）+ libsql（读 Turso）+ httpx（调桌面端），stdio transport
- 桌面端没跑时优雅降级（返回"请先启动 sushiro"，不报错）
- Turso 只读 token（`a:ro`）走环境变量，不进代码/git

### 安装（手动，本版）

git clone → `cd mcp && uv venv venv && uv pip install -e .` → 配 `claude_desktop_config.json`
（详见 `mcp/README.md`）

### 安全

- 全只读：不写数据、不调桌面端写接口、不暴露任意 SQL
- token 走 env / `.env`（gitignore）
- 桌面端只读 GET

### 不在本版

桌面端设置页"MCP 助手"一键开关（自动建 venv + 启停 + 注册到 Claude Desktop + 开机自启）——涉及 Go 启 Python 子进程、跨平台 venv/claude_config/自启，工作量大且有平台风险，**下个版本专门做**。本版用户手动装（README 给步骤）。

> 仅新增 `mcp/` 子目录（Python），桌面端 Go 代码零改动。
