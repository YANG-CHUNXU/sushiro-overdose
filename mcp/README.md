# sushiro MCP server

装到 AI agent（Claude Desktop / Cursor），让 AI 帮你用 sushiro：查排队数据、看你的预约状态、给到店建议、教你用工具。

## 它能干嘛

装好后，在 Claude Desktop / Cursor 里可以直接问：
- "太阳宫凯德店今天几点最挤？"
- "我想周六晚 6 点去太阳宫，该几点取号？"
- "我的凭证还有效吗？" / "我约了哪些时段？"
- "怎么拿通行证？"（AI 教你）

AI 会调用这个 MCP server 的工具（全只读）来回答。

## 12 个工具（全只读）

| 工具 | 作用 |
|------|------|
| `list_stores` | 搜门店（按城市/区域/店名） |
| `store_queue_history` | 某店历史叫号曲线（按历史几点叫到几号） |
| `store_pressure` | 某店各时段忙率（几点最挤） |
| `called_speed` | 某店叫号速度/吞吐率 |
| `compare_stores` | 多店对比 |
| `desktop_status` | 桌面端状态 + 凭证是否有效 |
| `my_reservations` | 我的预约 + 排队号 |
| `my_ticket_status` | 手里排队号实时状态 |
| `diagnose` | 环境诊断（凭证/证书/代理/网络）—— 排障利器 |
| `available_slots` | 可约时段 |
| `arrival_advice` | 综合到店建议（实时 ETA + 历史规律） |
| `explain_usage` | 教你用 sushiro |

## 前置

1. **Python ≥ 3.9**（开发/测试于 3.11）
2. **sushiro 桌面端在跑**（联动工具需要；查数据工具不需要桌面端，但要 Turso token）
3. **Turso 只读 token**：去 [Turso 控制台](https://turso.tech) 为 `su-shiro-ryujoxys` 库建一个**只读 token**（`a:ro`，不要用读写的）

## 安装

### 方式 A：从源码（git clone 后）

```bash
cd mcp
uv venv venv                          # 或 python3 -m venv venv
uv pip install -e .                   # 或 venv/bin/pip install -e .
```

### 配置 Claude Desktop

编辑 `~/Library/Application Support/Claude/claude_desktop_config.json`（macOS）或对应路径（Windows: `%APPDATA%\Claude\`），加：

```json
{
  "mcpServers": {
    "sushiro": {
      "command": "/绝对路径/mcp/venv/bin/python",
      "args": ["-m", "mcp_server"],
      "env": {
        "SUSHIRO_MCP_TURSO_URL": "libsql://su-shiro-ryujoxys.aws-us-west-2.turso.io",
        "SUSHIRO_MCP_TURSO_TOKEN": "你的只读token",
        "SUSHIRO_MCP_DESKTOP_PORT": "39871"
      }
    }
  }
}
```

重启 Claude Desktop，就能用了。

### 方式 B：Cursor

编辑 `~/.cursor/mcp.json`，同上结构。

## 配置项（环境变量）

| 变量 | 说明 |
|------|------|
| `SUSHIRO_MCP_TURSO_URL` | Turso 库地址（必填，查数据工具用） |
| `SUSHIRO_MCP_TURSO_TOKEN` | Turso **只读** token（必填，查数据工具用） |
| `SUSHIRO_MCP_DESKTOP_PORT` | 桌面端端口（默认 39871，联动工具用） |

桌面端没跑时，联动工具会返回"请先启动 sushiro"的友好提示（不报错）。

## 安全

- **全只读**：所有工具标 `readOnlyHint`，不写数据、不调桌面端写接口、不暴露任意 SQL
- token 只在环境变量 / `.env`（已 gitignore），不进代码
- 桌面端只读 GET（凭证/预约/诊断），不动账号

## 调试

```bash
# 用 mcp inspector 交互测试各工具
mcp dev mcp_server/server.py
```

## 后续

桌面端设置页的"MCP 助手"一键开关（自动建 venv + 启停 + 注册）正在开发，下个版本会有——届时不用手动配 claude_desktop_config.json。
