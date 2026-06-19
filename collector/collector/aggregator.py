"""聚合管线：从 queue_snapshots 聚合成 rollups。

产出三张表：
- store_bucket_rollups：时段聚合（压力 P50/P80 + 叫号 P20/P50/P80），worker 查它画图
- daily_store_bucket_rollups：按天细分的同结构
- called_intervals_rollups：叫号时序对的间隔/吞吐率（预测辅助）

核心分桶语义：
- store_bucket_rollups 主键 (store, date_type, weekday, bucket)，**每个 30min 桶一行**
- 同 date_type+weekday 的不同天会合并到同一行（如所有"工作日"的 17:30 合并）
- 同一天同一桶的多次采样先去重（取最后一条），避免高频放大单点
- 然后跨天合并：对该桶收集所有天的代表值，算 P50/P80（wait/groups）和 P20/P50/P80（called_no）
- busy_rate = 该桶"有排队"的样本比例
"""
from __future__ import annotations

import logging
from collections import defaultdict
from datetime import timedelta
from typing import Any, Dict, List, Optional, Tuple

from .datetype import date_type_for, half_hour_bucket, parse_iso_cst, snapshot_date
from .quantile import quantile
from .turso import TursoClient, as_int

log = logging.getLogger("collector.aggregate")

WRITE_BATCH = 200


def _load_holiday_sets(turso: TursoClient) -> Tuple[set, set]:
    holidays, workdays = set(), set()
    for r in turso.execute("SELECT date_key, date_type FROM holiday_calendar"):
        key, dt = r.get("date_key"), (r.get("date_type") or "").lower()
        if key and dt == "workday":
            workdays.add(key)
        elif key and dt == "holiday":
            holidays.add(key)
    return holidays, workdays


def aggregate_all(turso: TursoClient, days: Optional[int] = None) -> Dict[str, int]:
    """全量聚合。days=N 只聚合最近 N 天（None=全部）。"""
    holidays, workdays = _load_holiday_sets(turso)
    log.info("节假日配置：holiday=%d workday=%d", len(holidays), len(workdays))

    where = ""
    if days:
        rows = turso.execute("SELECT MAX(collected_at) AS mx FROM queue_snapshots")
        mx = rows[0].get("mx") if rows else None
        if mx:
            cutoff = parse_iso_cst(mx) - timedelta(days=days)
            where = f"WHERE collected_at >= '{cutoff.strftime('%Y-%m-%dT%H:%M:%S+08:00')}'"

    sql = (
        f"SELECT collected_at, store_id, wait_minutes, group_queues_count, "
        f"store_status, net_ticket_status, online_open, "
        f"display_called_no, dq_source, dq_anomaly FROM queue_snapshots {where} "
        f"ORDER BY store_id, collected_at"
    )
    rows = turso.execute(sql)
    log.info("读入 %d 条快照聚合", len(rows))

    parsed = [_parse_row(r) for r in rows]
    parsed = [p for p in parsed if p]

    rollups = _build_bucket_rollups(parsed, holidays, workdays)
    daily = _build_daily_rollups(parsed, holidays, workdays)
    intervals = _build_called_intervals(parsed, holidays, workdays)

    _write_bucket_rollups(turso, rollups)
    _write_daily_rollups(turso, daily)
    _write_intervals(turso, intervals)

    log.info(
        "✅ 聚合完成：rollups=%d daily=%d intervals=%d",
        len(rollups), len(daily), len(intervals),
    )
    return {"rollups": len(rollups), "daily": len(daily), "intervals": len(intervals)}


def _parse_row(r: Dict[str, Any]) -> Optional[dict]:
    try:
        dt = parse_iso_cst(r["collected_at"])
    except (KeyError, ValueError):
        return None
    called_raw = r.get("display_called_no")
    return {
        "dt": dt,
        "store_id": as_int(r.get("store_id")),
        "wait": as_int(r.get("wait_minutes")),
        "groups": as_int(r.get("group_queues_count")),
        "store_status": (r.get("store_status") or "").upper(),
        "online_open": as_int(r.get("online_open")),
        "called_no": as_int(called_raw) if called_raw not in (None, "") else None,
        "has_called": called_raw is not None and called_raw != "",
        "dq_anomaly": as_int(r.get("dq_anomaly")),
    }


def _confidence(n: int) -> str:
    if n >= 20:
        return "high"
    if n >= 8:
        return "medium"
    return "low"


def _build_bucket_rollups(
    parsed: List[dict], holidays: set, workdays: set
) -> List[dict]:
    """按 (store, date_type, weekday, bucket) 聚合。先同天同桶去重，再跨天合并算分位数。"""
    # 第一步：同 (store, date, bucket) 去重取最后一条（collected_at 最新）
    day_bucket_latest: Dict[tuple, dict] = {}
    for p in parsed:
        bucket = half_hour_bucket(p["dt"])
        sdate = snapshot_date(p["dt"])
        key = (p["store_id"], sdate, bucket)
        prev = day_bucket_latest.get(key)
        if prev is None or p["dt"] >= prev["dt"]:
            day_bucket_latest[key] = p

    # 第二步：按 (store, date_type, weekday, bucket) 收集所有天的去重帧
    grouped: Dict[tuple, List[dict]] = defaultdict(list)
    for (store_id, sdate, bucket), p in day_bucket_latest.items():
        date_type, weekday = date_type_for(p["dt"], holidays, workdays)
        grouped[(store_id, date_type, weekday, bucket)].append(p)

    out: List[dict] = []
    now_iso = parsed[-1]["dt"].isoformat() if parsed else ""
    updated_at = (now_iso or "")[:19] + "+08:00" if now_iso else ""
    for (store_id, date_type, weekday, bucket), frames in grouped.items():
        n = len(frames)
        waits = [f["wait"] for f in frames if f["wait"] > 0]
        groups_vals = [f["groups"] for f in frames if f["groups"] > 0]
        open_count = sum(1 for f in frames if f["store_status"] == "OPEN")
        online_count = sum(1 for f in frames if f["online_open"])
        busy_count = sum(1 for f in frames if f["wait"] > 0 or f["groups"] > 0)
        anomaly_count = sum(1 for f in frames if f["dq_anomaly"])

        # 叫号：只取 has_called 且 >0 的帧
        called_vals = [f["called_no"] for f in frames if f["has_called"] and (f["called_no"] or 0) > 0]

        out.append({
            "store_id": store_id,
            "date_type": date_type,
            "weekday": weekday,
            "time_bucket": bucket,
            "sample_count": n,
            "open_rate": open_count / n if n else 0.0,
            "online_open_rate": online_count / n if n else 0.0,
            "busy_rate": busy_count / n if n else 0.0,
            "wait_typical_minutes": quantile(waits, 0.5),
            "wait_safe_minutes": quantile(waits, 0.8),
            "wait_max_minutes": int(max(waits)) if waits else 0,
            "queue_groups_typical": quantile(groups_vals, 0.5),
            "queue_groups_safe": quantile(groups_vals, 0.8),
            "called_sample_count": len(called_vals),
            "called_no_slow": quantile(called_vals, 0.2),
            "called_no_typical": quantile(called_vals, 0.5),
            "called_no_fast": quantile(called_vals, 0.8),
            "dq_anomaly_rate": anomaly_count / n if n else 0.0,
            "confidence": _confidence(n),
            "updated_at": updated_at,
        })
    return out


def _build_daily_rollups(
    parsed: List[dict], holidays: set, workdays: set
) -> List[dict]:
    """按天细分：(snapshot_date, store, bucket)，同天同桶去重后直接取该天的代表值。"""
    day_bucket_latest: Dict[tuple, dict] = {}
    for p in parsed:
        bucket = half_hour_bucket(p["dt"])
        sdate = snapshot_date(p["dt"])
        key = (sdate, p["store_id"], bucket)
        prev = day_bucket_latest.get(key)
        if prev is None or p["dt"] >= prev["dt"]:
            day_bucket_latest[key] = p

    out: List[dict] = []
    for (sdate, store_id, bucket), f in day_bucket_latest.items():
        date_type, weekday = date_type_for(f["dt"], holidays, workdays)
        called = f["called_no"] if f["has_called"] and (f["called_no"] or 0) > 0 else None
        out.append({
            "snapshot_date": sdate,
            "store_id": store_id,
            "date_type": date_type,
            "weekday": weekday,
            "time_bucket": bucket,
            "sample_count": 1,
            "open_count": 1 if f["store_status"] == "OPEN" else 0,
            "online_open_count": 1 if f["online_open"] else 0,
            "busy_count": 1 if (f["wait"] > 0 or f["groups"] > 0) else 0,
            "open_rate": 1.0 if f["store_status"] == "OPEN" else 0.0,
            "online_open_rate": 1.0 if f["online_open"] else 0.0,
            "busy_rate": 1.0 if (f["wait"] > 0 or f["groups"] > 0) else 0.0,
            "wait_typical_minutes": float(f["wait"]) if f["wait"] > 0 else None,
            "wait_safe_minutes": float(f["wait"]) if f["wait"] > 0 else None,
            "wait_max_minutes": f["wait"],
            "queue_groups_typical": float(f["groups"]) if f["groups"] > 0 else None,
            "queue_groups_safe": float(f["groups"]) if f["groups"] > 0 else None,
            "called_sample_count": 1 if called is not None else 0,
            "called_no_slow": called,
            "called_no_typical": called,
            "called_no_fast": called,
            "confidence": "low",
            "updated_at": f["dt"].isoformat(),
        })
    return out


def _build_called_intervals(
    parsed: List[dict], holidays: set, workdays: set
) -> List[dict]:
    """叫号时序对：连续两帧（同店、时间递增）都 >0 且号递增 → 算 interval/delta。

    落在"前一个观测点的 bucket"（叫号推进发生时所在时段）。按 (store, date_type, bucket) 聚合。
    throughput = P50(delta) / P50(interval_seconds) * 3600
    capacity_utilization = throughput_per_hour / tables_capacity（capacity 在调用处补）
    """
    by_store: Dict[int, List[dict]] = defaultdict(list)
    for p in parsed:
        if p["has_called"] and (p["called_no"] or 0) > 0:
            by_store[p["store_id"]].append(p)

    grouped: Dict[tuple, List[dict]] = defaultdict(list)
    for store_id, frames in by_store.items():
        frames.sort(key=lambda x: x["dt"])
        for i in range(1, len(frames)):
            prev, cur = frames[i - 1], frames[i]
            delta = (cur["called_no"] or 0) - (prev["called_no"] or 0)
            if delta <= 0:
                continue  # 号没推进或倒退（倒退是异常，不计入正常间隔）
            interval_sec = (cur["dt"] - prev["dt"]).total_seconds()
            if interval_sec <= 0 or interval_sec > 6 * 3600:
                continue  # 间隔异常（>6h 多半跨了采集断档）
            date_type, _ = date_type_for(prev["dt"], holidays, workdays)
            bucket = half_hour_bucket(prev["dt"])
            grouped[(store_id, date_type, bucket)].append({
                "delta": delta, "interval": interval_sec,
            })

    out: List[dict] = []
    for (store_id, date_type, bucket), pairs in grouped.items():
        deltas = [p["delta"] for p in pairs]
        intervals = [p["interval"] for p in pairs]
        delta_p50 = quantile(deltas, 0.5) or 0
        interval_p50 = quantile(intervals, 0.5) or 0
        throughput = (delta_p50 / interval_p50 * 3600) if interval_p50 > 0 else None
        out.append({
            "store_id": store_id,
            "date_type": date_type,
            "time_bucket": bucket,
            "pair_count": len(pairs),
            "interval_typical_seconds": interval_p50,
            "delta_typical": delta_p50,
            "throughput_per_hour": throughput,
            "capacity_utilization": None,  # 写入时按 store 的 tables_capacity 补
        })
    return out


def _write_bucket_rollups(turso: TursoClient, rollups: List[dict]) -> None:
    if not rollups:
        return
    sql = """
    INSERT INTO store_bucket_rollups
      (store_id, date_type, weekday, time_bucket, sample_count,
       open_rate, online_open_rate, busy_rate,
       wait_typical_minutes, wait_safe_minutes, wait_max_minutes,
       queue_groups_typical, queue_groups_safe,
       called_sample_count, called_no_slow, called_no_typical, called_no_fast,
       dq_anomaly_rate, confidence, updated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(store_id, date_type, weekday, time_bucket) DO UPDATE SET
      sample_count=excluded.sample_count, open_rate=excluded.open_rate,
      online_open_rate=excluded.online_open_rate, busy_rate=excluded.busy_rate,
      wait_typical_minutes=excluded.wait_typical_minutes,
      wait_safe_minutes=excluded.wait_safe_minutes,
      wait_max_minutes=excluded.wait_max_minutes,
      queue_groups_typical=excluded.queue_groups_typical,
      queue_groups_safe=excluded.queue_groups_safe,
      called_sample_count=excluded.called_sample_count,
      called_no_slow=excluded.called_no_slow,
      called_no_typical=excluded.called_no_typical,
      called_no_fast=excluded.called_no_fast,
      dq_anomaly_rate=excluded.dq_anomaly_rate,
      confidence=excluded.confidence, updated_at=excluded.updated_at
    """
    args = [
        (
            r["store_id"], r["date_type"], r["weekday"], r["time_bucket"], r["sample_count"],
            r["open_rate"], r["online_open_rate"], r["busy_rate"],
            r["wait_typical_minutes"], r["wait_safe_minutes"], r["wait_max_minutes"],
            r["queue_groups_typical"], r["queue_groups_safe"],
            r["called_sample_count"], r["called_no_slow"], r["called_no_typical"],
            r["called_no_fast"], r["dq_anomaly_rate"], r["confidence"], r["updated_at"],
        )
        for r in rollups
    ]
    _batch_write(turso, sql, args)


def _write_daily_rollups(turso: TursoClient, daily: List[dict]) -> None:
    if not daily:
        return
    sql = """
    INSERT INTO daily_store_bucket_rollups
      (snapshot_date, store_id, date_type, weekday, time_bucket,
       sample_count, open_count, online_open_count, busy_count,
       open_rate, online_open_rate, busy_rate,
       wait_typical_minutes, wait_safe_minutes, wait_max_minutes,
       queue_groups_typical, queue_groups_safe,
       called_sample_count, called_no_slow, called_no_typical, called_no_fast,
       confidence, updated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(snapshot_date, store_id, time_bucket) DO UPDATE SET
      busy_rate=excluded.busy_rate, wait_typical_minutes=excluded.wait_typical_minutes,
      queue_groups_typical=excluded.queue_groups_typical,
      called_no_typical=excluded.called_no_typical, confidence=excluded.confidence,
      updated_at=excluded.updated_at
    """
    args = [
        (
            d["snapshot_date"], d["store_id"], d["date_type"], d["weekday"], d["time_bucket"],
            d["sample_count"], d["open_count"], d["online_open_count"], d["busy_count"],
            d["open_rate"], d["online_open_rate"], d["busy_rate"],
            d["wait_typical_minutes"], d["wait_safe_minutes"], d["wait_max_minutes"],
            d["queue_groups_typical"], d["queue_groups_safe"],
            d["called_sample_count"], d["called_no_slow"], d["called_no_typical"],
            d["called_no_fast"], d["confidence"], d["updated_at"],
        )
        for d in daily
    ]
    _batch_write(turso, sql, args)


def _write_intervals(turso: TursoClient, intervals: List[dict]) -> None:
    if not intervals:
        return
    # 取 store → tables_capacity 映射，补 capacity_utilization
    cap_map: Dict[int, int] = {}
    for r in turso.execute("SELECT store_id, tables_capacity FROM store_dimension"):
        cap_map[as_int(r.get("store_id"))] = as_int(r.get("tables_capacity"))

    sql = """
    INSERT INTO called_intervals_rollups
      (store_id, date_type, time_bucket, pair_count,
       interval_typical_seconds, delta_typical, throughput_per_hour,
       capacity_utilization, updated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(store_id, date_type, time_bucket) DO UPDATE SET
      pair_count=excluded.pair_count,
      interval_typical_seconds=excluded.interval_typical_seconds,
      delta_typical=excluded.delta_typical,
      throughput_per_hour=excluded.throughput_per_hour,
      capacity_utilization=excluded.capacity_utilization, updated_at=excluded.updated_at
    """
    args = []
    now_iso = intervals[0].get("updated_at") or ""
    from .collector import _fmt_dt
    updated = _fmt_dt()
    for it in intervals:
        cap = cap_map.get(it["store_id"], 0)
        util = (it["throughput_per_hour"] / cap) if (cap > 0 and it["throughput_per_hour"]) else None
        args.append(
            (
                it["store_id"], it["date_type"], it["time_bucket"], it["pair_count"],
                it["interval_typical_seconds"], it["delta_typical"], it["throughput_per_hour"],
                util, updated,
            )
        )
    _batch_write(turso, sql, args)


def _batch_write(turso: TursoClient, sql: str, args: List[tuple]) -> None:
    for i in range(0, len(args), WRITE_BATCH):
        batch = args[i : i + WRITE_BATCH]
        turso.execute_many([(sql, a) for a in batch])
