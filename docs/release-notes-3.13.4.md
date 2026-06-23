# v3.13.4

## docs: 采集器 LLM 部署指南

新增 `collector/AGENTS.md` —— 专为 AI agent 写的采集器部署指南，零猜测照着做。

### 为什么

要把采集器部署到别的服务器，让 LLM 帮忙部署。现有 README 是给人看的，AGENTS.md 是给 LLM 看的：
结构化、按顺序、每步有验证检查点、故障排查表、关键设计说明。

### 内容

- 前置条件（Python ≥ 3.9、Turso 读写 token、网络）
- 10 步部署流程（clone → venv → config → init-schema → seed-holidays → bootstrap → migrate-old → run-once → aggregate → systemd），每步带验证命令
- 命令一览表 + 配置项说明
- 故障排查表（401/网络/systemd 秒退等常见问题）
- 关键设计说明（叫号vs压力同表、两接口、聚合30天、脏数据过滤、schema 兼容桌面端）
- 部署后验证采集在跑的命令

> 仅新增文档，无代码改动。
