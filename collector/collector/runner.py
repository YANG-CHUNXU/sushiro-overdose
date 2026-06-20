"""常驻运行循环。

每 interval_seconds 采一轮（营业时段内）。每天凌晨：
- 02:00 跑一次全量聚合（产出 rollups + 叫号三档 + 间隔/吞吐）
- 03:00 跑一次归档（裁剪超期原始快照）

优雅退出：SIGTERM/SIGINT 触发后完成当前轮再退出（systemd restart 不丢数据）。
"""
from __future__ import annotations

import logging
import signal
import threading
import time
from datetime import datetime, timedelta, timezone
from typing import Any, Dict

from .aggregator import aggregate_all
from .archive import archive_old
from .collector import collect_once

log = logging.getLogger("collector.run")

CST = timezone(timedelta(hours=8))

_STOP = threading.Event()


def _handle_signal(signum, _frame):
    log.info("收到信号 %s，准备退出（完成当前轮）", signum)
    _STOP.set()


def run_loop(cfg: Dict[str, Any]) -> None:
    signal.signal(signal.SIGTERM, _handle_signal)
    signal.signal(signal.SIGINT, _handle_signal)

    coll_cfg = cfg.get("collect", {})
    interval = int(coll_cfg.get("interval_seconds", 900))
    active_hours = coll_cfg.get("active_hours", [10, 22])
    retention = int(cfg.get("archive", {}).get("retention_days", 60))

    last_aggregate_date = ""
    last_archive_date = ""

    log.info(
        "采集器启动：interval=%ds active=%s retention=%dd",
        interval, active_hours, retention,
    )

    while not _STOP.is_set():
        now = datetime.now(CST)

        # 营业时段判断（active_hours=[10,22] 表示 10≤hour<22 才采）
        if active_hours and len(active_hours) == 2:
            lo, hi = active_hours
            if not (lo <= now.hour < hi):
                log.debug("非营业时段 %s，跳过采集", now.strftime("%H:%M"))
                # 睡到下一个采集点或退出
                _STOP.wait(interval)
                continue

        # 跑一轮采集
        try:
            collect_once(cfg)
        except Exception as e:
            log.error("采集轮失败（下轮重试）: %s", e)

        # 每日聚合（02:00 之后、且今天还没聚合过）
        today = now.strftime("%Y-%m-%d")
        if now.hour >= 2 and last_aggregate_date != today:
            log.info("每日聚合 %s", today)
            try:
                from .turso import TursoClient
                from .config import require_credential

                turso = TursoClient(
                    require_credential(cfg, "turso", "url"),
                    require_credential(cfg, "turso", "auth_token"),
                )
                # 只聚合最近 30 天：恒定读量，不随历史快照增长而爆炸（全量重算会触及 Turso 额度）。
                # rollups 行覆盖所有历史桶（upsert 合并），所以 30 天样本足够算分位数且覆盖全天各时段。
                aggregate_all(turso, days=30)
                last_aggregate_date = today
            except Exception as e:
                log.error("聚合失败: %s", e)

        # 每日归档（03:00 之后、且今天还没归档过）
        if now.hour >= 3 and last_archive_date != today:
            log.info("每日归档 %s", today)
            try:
                from .turso import TursoClient
                from .config import require_credential

                turso = TursoClient(
                    require_credential(cfg, "turso", "url"),
                    require_credential(cfg, "turso", "auth_token"),
                )
                archive_old(turso, retention)
                last_archive_date = today
            except Exception as e:
                log.error("归档失败: %s", e)

        # 等下一轮（可被信号中断）
        _STOP.wait(interval)

    log.info("采集器已退出")
