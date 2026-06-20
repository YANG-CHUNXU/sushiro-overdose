# v3.10.2

## fix: 采集器部署前审查修复（runner 死代码 / systemd 超时 / 文档）

发版前并行审查（代码审查 + 数据质量 + 部署文档）发现几个影响换机部署的问题，本次修复。

### mustFix

1. **runner 每日聚合/归档是死代码**（`runner.py`）
   原循环结构里，营业时段判断 `if not (10<=hour<22): continue` 在 02:00/03:00 生效，
   把后面的每日聚合/归档分支跳过了——实际聚合推迟到 10:00 才跑，与文档说的"02:00 聚合"不符。
   修复：重构循环，聚合/归档移到营业时段判断**之前**（凌晨照常跑），营业时段判断只作用于采集。
   顺带加 `active_hours` 的 `lo<hi` 校验（误配 `[22,10]` 会死循环）。

2. **systemd `TimeoutStopSec=60` 太短**（`collector.service`）
   一轮采集最坏 ~200 店 × 15s / concurrency 8 ≈ 375s，SIGTERM 60s 后硬杀会让 Turso 批量写
   到一半被中断（store_latest 与 snapshots 不一致）。改为 600s。

### niceToHave

3. **daily rollups 脏值过滤**（`aggregator.py`）
   `_build_daily_rollups` 原来不做 cap/脏值过滤，与 `_build_bucket_rollups` 不一致。统一加
   `wait>cap` / `groups>200` 过滤。

4. **requirements 去掉无用的 requests**（`requirements.txt`）
   代码全用 urllib，零外部依赖。注明需 Python ≥ 3.9。

5. **README 部署文档补充**：Python ≥ 3.9 要求；`scp -r` 会带 macOS venv 到 Linux 不可用，
   改用 `rsync --exclude venv`；systemd 默认 root 运行的提示。

> 仅改 `collector/`（Python + 文档），桌面端 Go 代码无改动。
