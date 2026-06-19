"""归档：删除超过保留期的原始 queue_snapshots（已聚合进 rollups）。

原始快照会无限增长（每 15min × 118 店 ≈ 每天约 11000 行），必须定期裁剪。
保留期默认 60 天，足够聚合统计 + 回溯近期异常。更老的靠 daily_store_bucket_rollups。
"""
from __future__ import annotations

import logging
from typing import Any, Dict

from .collector import _fmt_dt
from .turso import TursoClient, as_int

log = logging.getLogger("collector.archive")


def archive_old(turso: TursoClient, retention_days: int = 60) -> Dict[str, int]:
    """删除 retention_days 天前的 queue_snapshots。返回 {deleted, remaining}。"""
    # 找最新快照时间，往前推 retention_days 作为裁剪线
    rows = turso.execute("SELECT MAX(collected_at) AS mx FROM queue_snapshots")
    mx = rows[0].get("mx") if rows else None
    if not mx:
        log.info("无快照可归档")
        return {"deleted": 0, "remaining": 0}

    from datetime import timedelta
    from .datetype import parse_iso_cst

    cutoff_dt = parse_iso_cst(mx) - timedelta(days=retention_days)
    cutoff = cutoff_dt.strftime("%Y-%m-%dT%H:%M:%S+08:00")

    # 先统计要删多少
    cnt_rows = turso.execute(
        "SELECT COUNT(*) AS c FROM queue_snapshots WHERE collected_at < ?", (cutoff,)
    )
    to_delete = as_int(cnt_rows[0].get("c")) if cnt_rows else 0

    # 删除（分批避免单次过大事务；这里直接删，量级可控）
    turso.execute("DELETE FROM queue_snapshots WHERE collected_at < ?", (cutoff,))

    remain_rows = turso.execute("SELECT COUNT(*) AS c FROM queue_snapshots")
    remaining = as_int(remain_rows[0].get("c")) if remain_rows else 0

    log.info("归档：删除 %d 行（< %s），剩余 %d 行", to_delete, cutoff, remaining)

    # 写归档日志
    _record_archive(turso, to_delete, remaining, retention_days, cutoff)
    return {"deleted": to_delete, "remaining": remaining}


def _record_archive(
    turso: TursoClient, deleted: int, remaining: int, retention: int, cutoff: str
) -> None:
    now = _fmt_dt()
    sql = """
    INSERT INTO archive_runs
      (archive_date, snapshots, rollups_written, global_rollups,
       raw_deleted, raw_remaining, retention_days, prune_before, ok,
       error_message, created_at, updated_at)
    VALUES (?, ?, 0, 0, ?, ?, ?, ?, 1, '', ?, ?)
    """
    from datetime import datetime
    try:
        turso.execute(
            sql,
            (
                now[:10], deleted, deleted, remaining, retention, cutoff,
                now, now,
            ),
        )
    except Exception as e:
        log.warning("写 archive_runs 失败（不影响归档）: %s", e)
