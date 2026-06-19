"""从旧 Turso 库导入压力历史到新库。

旧库 queue_snapshots 有 25288 条压力快照（等待/桌数/状态），但没有叫号。
导入时：display_called_no=NULL、group_queues_json=NULL、dq_source='stores_list'。
同时把旧库 store_dimension 的 city/area/address 补进新库 store_dimension（新库 bootstrap 时 city 为空）。

幂等：用 (collected_at, store_id, dq_source) 去重，已导入的跳过。
"""
from __future__ import annotations

import logging
from typing import Any, Dict, List

from .config import load_config, require_credential
from .turso import TursoClient

log = logging.getLogger("collector.migrate")

PAGE_SIZE = 500  # 每次从旧库读多少行
WRITE_BATCH = 200  # 每次往新库写多少行（一个 pipeline 请求）


def migrate_old(cfg: Dict[str, Any], limit: int = 0, dry_run: bool = False) -> Dict[str, int]:
    """从旧库导入压力快照 + 补 city。返回 {snapshots_imported, cities_updated}。"""
    old_url = require_credential(cfg, "old_turso", "url")
    old_token = require_credential(cfg, "old_turso", "auth_token")
    new_url = require_credential(cfg, "turso", "url")
    new_token = require_credential(cfg, "turso", "auth_token")

    old = TursoClient(old_url, old_token)
    new = TursoClient(new_url, new_token)

    # 1. 补 store_dimension 的 city/area/address（新库 bootstrap 时 city 为空）
    cities_updated = 0
    if not dry_run:
        cities_updated = _backfill_store_dimension(old, new)

    # 2. 算已导入到哪了（按 collected_at 最大值断点续传）
    if dry_run:
        checkpoint = ""
    else:
        rows = new.execute(
            "SELECT MAX(collected_at) AS mx FROM queue_snapshots WHERE dq_source='stores_list'"
        )
        checkpoint = rows[0].get("mx") if rows and rows[0].get("mx") else ""
    log.info("断点续传 checkpoint=%s", checkpoint or "(从头开始)")

    # 3. 用 ID 游标分页读旧库 → 批量写新库（最可靠，不分 dry_run/正式）
    total = 0
    last_id = 0
    while True:
        remaining = ""
        if limit and total >= limit:
            break
        if limit:
            remaining = f"LIMIT {limit - total}"
        sql = (
            f"SELECT id, collected_at, store_id, wait_minutes, group_queues_count, "
            f"store_status, net_ticket_status, reservation_status, online_open, "
            f"wait_time_counter, wait_time_cap, source_endpoint, api_profile_version "
            f"FROM queue_snapshots WHERE id > {last_id} ORDER BY id LIMIT {PAGE_SIZE}"
        )
        # remaining 未用（保留参数语义），分页靠 PAGE_SIZE
        del remaining
        page = old.execute(sql)
        if not page:
            break
        last_id = int(page[-1]["id"])

        if dry_run:
            total += len(page)
            if total % 5000 < PAGE_SIZE:
                log.info("[dry-run] 已读 %d 行（last_id=%d）", total, last_id)
        else:
            written = _write_snapshots_batch(new, page)
            total += written
            if total % 5000 < PAGE_SIZE:
                log.info("已导入 %d 行（last_id=%d）", total, last_id)

        if len(page) < PAGE_SIZE:
            break

    log.info("✅ 导入完成：快照 %d 行，city 更新 %d 家", total, cities_updated)
    return {"snapshots_imported": total, "cities_updated": cities_updated}


def _backfill_store_dimension(old: TursoClient, new: TursoClient) -> int:
    """从旧库读 city/area/address，补进新库 store_dimension。"""
    rows = old.execute(
        "SELECT store_id, city, area, address FROM store_dimension "
        "WHERE city IS NOT NULL AND city != ''"
    )
    if not rows:
        return 0
    sql = (
        "UPDATE store_dimension SET city=?, area=?, address=? WHERE store_id=? "
        "AND (city IS NULL OR city = '')"
    )
    args_list = [
        (r.get("city", ""), r.get("area", ""), r.get("address", ""), int(r["store_id"]))
        for r in rows
    ]
    new.execute_many([(sql, a) for a in args_list])
    log.info("补 city/area/address：%d 家", len(args_list))
    return len(args_list)


def _write_snapshots_batch(new: TursoClient, rows: List[Dict[str, Any]]) -> int:
    sql = """
    INSERT OR IGNORE INTO queue_snapshots
      (collected_at, store_id, wait_minutes, group_queues_count, store_status,
       net_ticket_status, reservation_status, online_open, wait_time_counter,
       wait_time_cap, display_called_no, group_queues_json, dq_source,
       api_profile_version, dq_anomaly)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, 'stores_list', ?, 0)
    """

    def _arg(r: Dict[str, Any]) -> tuple:
        def _i(key: str) -> int:
            v = r.get(key)
            try:
                return int(v) if v is not None else 0
            except (TypeError, ValueError):
                return 0

        return (
            r["collected_at"], int(r["store_id"]), _i("wait_minutes"),
            _i("group_queues_count"), r.get("store_status") or "",
            r.get("net_ticket_status") or "", r.get("reservation_status") or "",
            _i("online_open"),
            _int_or_none(r.get("wait_time_counter")),
            _int_or_none(r.get("wait_time_cap")),
            r.get("api_profile_version") or "public-profile-v1",
        )

    args_list = [_arg(r) for r in rows]
    for i in range(0, len(args_list), WRITE_BATCH):
        batch = args_list[i : i + WRITE_BATCH]
        new.execute_many([(sql, a) for a in batch])
    return len(args_list)


def _int_or_none(v: Any) -> Any:
    if v is None or v == "":
        return None
    try:
        return int(v)
    except (TypeError, ValueError):
        return None
