"""一轮采集：stores?（全国压力）+ 每店 getStoreById?（叫号）→ 写 Turso。

数据流：
  1. list_stores → 全国门店压力快照（无叫号，写 dq_source='stores_list' 帧）
  2. 对每店 get_store → 叫号（groupQueues），写 dq_source='store_detail' 帧
  3. upsert store_latest（取单店帧优先，没有则列表帧；叫号列只单店帧有）
  4. upsert store_dimension（门店静态信息）

注意：同一家店一轮里会有两帧（列表帧 + 单店帧），这是有意的——列表帧覆盖全国压力，
单店帧补充叫号。聚合层按 dq_source 区分取样本。
"""
from __future__ import annotations

import logging
import uuid
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timedelta, timezone
from typing import Any, Dict, List, Optional, Tuple

from .config import load_config, require_credential
from .models import Snapshot, StoreInfo
from .sushiro_client import SushiroClient
from .turso import TursoClient

log = logging.getLogger("collector.collect")

# 门店时区固定 CST(+08)，与主项目 slot.go SushiroTimezone 一致，避免容器 TZ=UTC 劈桶
CST = timezone(offset=timedelta(hours=8))


def now_cst_iso() -> str:
    return datetime.now(CST).isoformat(timespec="seconds")


def _fmt_dt() -> str:
    # 单独函数，方便测试 monkeypatch
    return now_cst_iso()


def collect_once(
    cfg: Optional[Dict[str, Any]] = None,
    *,
    store_id_filter: Optional[List[int]] = None,
    skip_store_detail: bool = False,
) -> Dict[str, int]:
    """跑一轮采集。返回统计 {stores_seen, snapshots_written, detail_ok, detail_fail}。"""
    cfg = cfg or load_config()
    token = require_credential(cfg, "sushiro", "token")
    turso_url = require_credential(cfg, "turso", "url")
    turso_token = require_credential(cfg, "turso", "auth_token")

    coll_cfg = cfg.get("collect", {})
    client = SushiroClient(
        token=token,
        base_url=cfg["sushiro"].get("base_url"),
        referer=cfg["sushiro"].get("referer"),
        user_agent=cfg["sushiro"].get("user_agent"),
        timeout=float(coll_cfg.get("per_call_timeout_seconds", 15)),
    )
    turso = TursoClient(turso_url, turso_token)

    run_id = uuid.uuid4().hex[:16]
    started_at = _fmt_dt()
    log.info("采集开始 run_id=%s", run_id)

    # 1. 列表接口：全国压力
    lat = float(coll_cfg.get("list_latitude", 23.13))
    lng = float(coll_cfg.get("list_longitude", 113.26))
    try:
        list_stores = client.list_stores(lat, lng)
    except Exception as e:
        log.error("列表接口失败: %s", e)
        _record_run(turso, run_id, started_at, "stores+detail", 0, 0, False, str(e))
        raise
    log.info("列表接口拿到 %d 家店", len(list_stores))

    # 门店筛选
    if store_id_filter is not None:
        wanted = set(store_id_filter)
        list_stores = [s for s in list_stores if s.store_id in wanted]
    elif coll_cfg.get("store_ids"):
        wanted = set(int(x) for x in coll_cfg["store_ids"])
        list_stores = [s for s in list_stores if s.store_id in wanted]

    collected_at = _fmt_dt()
    snapshots: List[Snapshot] = []
    store_by_id: Dict[int, StoreInfo] = {}

    # 列表帧（无叫号）
    for s in list_stores:
        store_by_id[s.store_id] = s
        snapshots.append(
            Snapshot.from_store(s, collected_at, dq_source="stores_list", include_called=False)
        )

    detail_ok = 0
    detail_fail = 0
    detail_results: Dict[int, Optional[StoreInfo]] = {}

    # 2. 单店接口：叫号
    if not skip_store_detail:
        concurrency = int(coll_cfg.get("concurrency", 8))
        ids = [s.store_id for s in list_stores if s.store_id > 0]
        with ThreadPoolExecutor(max_workers=concurrency) as ex:
            futures = {ex.submit(client.get_store, sid): sid for sid in ids}
            for fut in as_completed(futures):
                sid = futures[fut]
                try:
                    detail_results[sid] = fut.result()
                    detail_ok += 1
                except Exception as e:
                    detail_fail += 1
                    log.warning("getStoreById %d 失败: %s", sid, e)

        # 单店帧（带叫号）。单店接口可能没返回（关店/找不到）→ 跳过，列表帧已兜底
        detail_at = _fmt_dt()
        for sid, detail in detail_results.items():
            if detail is None:
                continue
            # 合并：单店帧可能缺列表帧有的静态字段（如 city），从列表帧补
            base = store_by_id.get(sid)
            if base:
                detail = _merge_store(base, detail)
                store_by_id[sid] = detail
            snapshots.append(
                Snapshot.from_store(detail, detail_at, dq_source="store_detail", include_called=True)
            )

    # 3 & 4. 写库
    _write_snapshots(turso, snapshots)
    _upsert_store_latest(turso, list(store_by_id.values()), collected_at)
    _upsert_store_dimension(turso, list(store_by_id.values()), collected_at)

    stats = {
        "stores_seen": len(list_stores),
        "snapshots_written": len(snapshots),
        "detail_ok": detail_ok,
        "detail_fail": detail_fail,
    }
    _record_run(
        turso,
        run_id,
        started_at,
        "stores+detail",
        len(list_stores),
        len(snapshots),
        True,
        "",
    )
    log.info("采集完成 run_id=%s %s", run_id, stats)
    return stats


def _merge_store(base: StoreInfo, detail: StoreInfo) -> StoreInfo:
    """单店帧缺的静态字段从列表帧补。"""
    if not detail.city and base.city:
        detail.city = base.city
    if not detail.area and base.area:
        detail.area = base.area
    if not detail.address and base.address:
        detail.address = base.address
    if detail.tables_capacity == 0 and base.tables_capacity:
        detail.tables_capacity = base.tables_capacity
    if detail.counters_capacity == 0 and base.counters_capacity:
        detail.counters_capacity = base.counters_capacity
    if not detail.open_date and base.open_date:
        detail.open_date = base.open_date
    if detail.latitude is None and base.latitude is not None:
        detail.latitude = base.latitude
    if detail.longitude is None and base.longitude is not None:
        detail.longitude = base.longitude
    return detail


def _write_snapshots(turso: TursoClient, snapshots: List[Snapshot]) -> None:
    if not snapshots:
        return
    sql = """
    INSERT INTO queue_snapshots
      (collected_at, store_id, wait_minutes, group_queues_count, store_status,
       net_ticket_status, reservation_status, online_open, wait_time_counter,
       wait_time_cap, display_called_no, group_queues_json, dq_source,
       api_profile_version, dq_anomaly)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    """
    # 分批，每批 200 条（避免单请求过大）
    BATCH = 200
    for i in range(0, len(snapshots), BATCH):
        batch = snapshots[i : i + BATCH]
        stmts = [(_snapshot_args(s)) for s in batch]
        # execute_many 接受 (sql, params) 但每条 sql 相同 → 用 args 列表形式
        turso.execute_many([(sql, a) for a in stmts])


def _snapshot_args(s: Snapshot) -> tuple:
    return (
        s.collected_at,
        s.store_id,
        s.wait_minutes,
        s.group_queues_count,
        s.store_status,
        s.net_ticket_status,
        s.reservation_status,
        s.online_open,
        s.wait_time_counter,
        s.wait_time_cap,
        s.display_called_no,  # None → NULL
        s.group_queues_json,
        s.dq_source,
        s.api_profile_version,
        s.dq_anomaly,
    )


def _upsert_store_latest(turso: TursoClient, stores: List[StoreInfo], collected_at: str) -> None:
    if not stores:
        return
    updated_at = _fmt_dt()
    sql = """
    INSERT INTO store_latest
      (store_id, collected_at, name, city, area, wait_minutes, group_queues_count,
       store_status, net_ticket_status, reservation_status, online_open,
       wait_time_counter, wait_time_cap, display_called_no, group_queues_json, updated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(store_id) DO UPDATE SET
      collected_at=excluded.collected_at, name=excluded.name, city=excluded.city,
      area=excluded.area, wait_minutes=excluded.wait_minutes,
      group_queues_count=excluded.group_queues_count, store_status=excluded.store_status,
      net_ticket_status=excluded.net_ticket_status,
      reservation_status=excluded.reservation_status, online_open=excluded.online_open,
      wait_time_counter=excluded.wait_time_counter, wait_time_cap=excluded.wait_time_cap,
      display_called_no=excluded.display_called_no,
      group_queues_json=excluded.group_queues_json, updated_at=excluded.updated_at
    """
    args_list = []
    for s in stores:
        import json as _json
        gq_json = _json.dumps(s.group_queues, ensure_ascii=False) if s.group_queues else None
        # store_latest 的叫号：单店帧才有（group_queues 非 None）。列表帧 store 的 group_queues
        # 是 None → display_called_no 存 NULL（与"没取叫号"语义一致）。
        called_no = None
        if s.group_queues is not None:
            from .called_no import current_called_no
            called_no = current_called_no(s.group_queues)
        args_list.append(
            (
                s.store_id, collected_at, s.name, s.city, s.area, s.wait_minutes,
                s.group_queues_count, s.store_status, s.net_ticket_status,
                s.reservation_status, 1 if s.online_open else 0,
                s.wait_time_counter, s.wait_time_cap, called_no, gq_json, updated_at,
            )
        )
    turso.execute_many([(sql, a) for a in args_list])


def _upsert_store_dimension(turso: TursoClient, stores: List[StoreInfo], now_iso: str) -> None:
    if not stores:
        return
    sql = """
    INSERT INTO store_dimension
      (store_id, name, city, area, address, latitude, longitude, open_date,
       tables_capacity, counters_capacity, first_seen_at, last_seen_at, is_active)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
    ON CONFLICT(store_id) DO UPDATE SET
      name=excluded.name, city=excluded.city, area=excluded.area, address=excluded.address,
      latitude=COALESCE(excluded.latitude, store_dimension.latitude),
      longitude=COALESCE(excluded.longitude, store_dimension.longitude),
      open_date=excluded.open_date,
      tables_capacity=CASE WHEN excluded.tables_capacity>0 THEN excluded.tables_capacity ELSE store_dimension.tables_capacity END,
      counters_capacity=CASE WHEN excluded.counters_capacity>0 THEN excluded.counters_capacity ELSE store_dimension.counters_capacity END,
      last_seen_at=excluded.last_seen_at, is_active=1
    """
    args_list = []
    for s in stores:
        args_list.append(
            (
                s.store_id, s.name, s.city, s.area, s.address, s.latitude, s.longitude,
                s.open_date or None, s.tables_capacity, s.counters_capacity,
                now_iso, now_iso,
            )
        )
    turso.execute_many([(sql, a) for a in args_list])


def _record_run(
    turso: TursoClient,
    run_id: str,
    started_at: str,
    endpoint: str,
    stores_seen: int,
    records_written: int,
    ok: bool,
    error_message: str,
) -> None:
    finished_at = _fmt_dt()
    sql = """
    INSERT INTO collector_runs
      (run_id, started_at, finished_at, endpoint, stores_seen, records_written,
       ok, error_message, api_profile_version)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(run_id) DO UPDATE SET
      finished_at=excluded.finished_at, stores_seen=excluded.stores_seen,
      records_written=excluded.records_written, ok=excluded.ok,
      error_message=excluded.error_message
    """
    try:
        turso.execute(
            sql,
            (
                run_id, started_at, finished_at, endpoint, stores_seen,
                records_written, 1 if ok else 0, error_message, "public-profile-v1",
            ),
        )
    except Exception as e:
        log.warning("写 collector_runs 失败（不影响采集）: %s", e)
