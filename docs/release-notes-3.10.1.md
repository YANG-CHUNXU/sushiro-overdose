# v3.10.1

## fix: 采集器聚合过滤脏数据 + 增量聚合省 Rows

承接 v3.10.0 的采集器，修两个问题。

### 1. 脏数据污染 rollups

聚合产出的 `store_bucket_rollups.wait_typical_minutes` 出现荒谬值（如 store 38 weekday
18:30 = 1430 分钟≈24 小时）。核实根因是**旧库的 `wait_minutes` 有接口异常脏数据**（超过
寿司郎系统封顶 `wait_time_cap=180`），聚合忠实反映了它。

修复：聚合时丢弃脏值——
- `wait_minutes > wait_time_cap`（180）一律丢弃，分位数/最大值/busy_rate 都不计入
- `group_queues_count > 200` 丢弃（实测脏值到 529，单店不可能同时排这么多桌）

过滤后 store 38 高峰从 1430min 修正到合理的 165min。

### 2. 聚合 Rows 消耗优化

`aggregate_all` 原本是全量重算——每天把所有历史快照重读、所有 rollups 重写。快照随时间增长
后，单次聚合的 Rows Read 会线性爆炸，可能触及 Turso 月额度。

修复：runner 每日聚合改为**只读最近 30 天**（`aggregate_all(days=30)`），读量恒定不增长。
rollups 行仍覆盖所有历史时段桶（upsert 合并），30 天样本足够算分位数。

稳态估算：日消耗 ~90k read / ~51k written，占 Turso 免费额度（1B read / 25M written）的 6%。

> 仅改 `collector/`（Python），桌面端 Go 代码无改动。
