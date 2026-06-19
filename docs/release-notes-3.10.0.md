# v3.10.0

## 新增：Turso 采集器（叫号 + 压力 + 预测辅助字段）

新增独立 Python 采集器 `collector/`，修复线上数据库"有压力数据、零叫号数据"的根因。

### 背景

旧采集项目只调寿司郎批量列表接口 `stores?`，该接口**不返回叫号**（叫号只在单店接口
`getStoreById?` 的 `groupQueues` 里）。导致旧 Turso 库有 2.5 万条"排队压力"快照（等待分钟、
桌数、忙率），但**零条叫号数据**——所以桌面端"按历史几点叫到几号"的叫号曲线永远是空的。

### 本次

新采集器每轮既调 `stores?`（全国压力）又对每店调 `getStoreById?`（拿叫号），补上叫号，并新增
预测辅助字段。

- **语言/形态**：Python，纯标准库（urllib），单包可独立部署，systemd 自启自愈。
- **频率**：线上每 15 分钟一轮（营业时段 10-22 点）；本机桌面端仍是高频采集。
- **范围**：全国 116+ 家店全采叫号；`store_ids` 可配缩范围。
- **新库 schema**：9 张表，叫号与压力同表分层（`dq_source` + `display_called_no IS NULL`
  区分"没取叫号"vs"取了但无堂食叫号"）。
- **辅助预测字段**：叫号间隔/吞吐率（`called_intervals_rollups`）、节假日日历
  （`holiday_calendar`）、数据质量标记（`dq_anomaly`/`dq_anomaly_rate`）。
- **兼容性**：`store_bucket_rollups` 表名 + 列名（含 `called_no_slow/typical/fast`）与桌面端
  Cloudflare Worker 完全兼容——**桌面端 Go 代码零改动**，Worker 重新部署指向新库即可读到叫号。
- **安全**：token/Turso 凭证走 `config.json` + 环境变量，绝不进代码或 git（`config.example.json`
  做模板，真实配置已 gitignore）。
- **迁移**：`collector/README.md` 含完整部署 + 迁移到其他服务器的步骤；旧库 25288 条压力历史
  可用 `migrate-old` 命令导入新库（叫号列 NULL，叫号从部署后开始攒）。

### 命令

```
init-schema      建全部表
seed-holidays    初始化节假日日历
bootstrap        冷启动门店维度
migrate-old      从旧库导入压力历史
run-once         跑一轮采集（验证）
aggregate-now    立即聚合（产出 rollups + 叫号三档）
run              常驻运行（每 15min 采集 + 每日聚合/归档）
```

详见 `collector/README.md`。

### 桌面端用户须知

本次桌面端 Go 代码无改动，现有版本直接可用。要看到新的线上叫号数据，需将 Cloudflare
Worker 的 `TURSO_DATABASE_URL` secret 指向新库并重新部署（采集器部署 + 跑几天攒出叫号样本后）。

## 其他

排队预测算法延续上轮（v3.9）的实时权重保底 + 官方等待不抬个人下界优化，已包含在工作树中。
