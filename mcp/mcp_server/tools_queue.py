"""查排队数据 tools（读 Turso 全国历史）。

每个函数返回裁剪好的摘要（给 AI 看的，不是原始 row）。
全只读。日期类型可选，默认查 weekday（工作日最常见）。
"""
from __future__ import annotations

from typing import Any, Dict, List, Optional

from .turso import TursoClient

# time_bucket 营业时段范围（10:00-22:00），过滤掉关店时段的噪声桶
DAY_BUCKETS_RANGE = ("10:00", "22:00")


def _bucket_in_range(b: str) -> bool:
    return DAY_BUCKETS_RANGE[0] <= b <= DAY_BUCKETS_RANGE[1]


def list_stores(
    turso: TursoClient, city: Optional[str] = None, q: Optional[str] = None, limit: int = 20
) -> List[Dict[str, Any]]:
    """搜门店。city 匹配 city 或 area（area 含城市信息，如"广州天河区"）；q 模糊匹配 name/city/area。"""
    sql = "SELECT store_id, name, city, area FROM store_dimension WHERE is_active=1"
    params: list = []
    if city:
        # city 字段可能为空（bootstrap 不填 city），用 area 兜底（area 格式"城市+区"）
        sql += " AND (city LIKE ? OR area LIKE ?)"
        params += [f"%{city}%", f"%{city}%"]
    if q:
        sql += " AND (name LIKE ? OR city LIKE ? OR area LIKE ?)"
        params += [f"%{q}%", f"%{q}%", f"%{q}%"]
    sql += " ORDER BY store_id LIMIT ?"
    params.append(limit)
    rows = turso.query(sql, tuple(params))
    return [
        {"store_id": r["store_id"], "name": r["name"], "city": r["city"] or "", "area": r["area"] or ""}
        for r in rows
    ]


def store_queue_history(
    turso: TursoClient, store_id: int, date_type: str = "weekday"
) -> Dict[str, Any]:
    """某店某 date_type 的历史叫号曲线 + 忙率 + 等位。

    返回各时段（营业时段 10:00-22:00）：典型叫到几号、保守/偏快、忙率、等位分钟、在等桌数。
    rollups 按 (date_type, weekday, bucket) 分，这里 GROUP BY bucket 跨 weekday 平均（周一~周五差异小）。
    这是"按历史，几点叫到几号"的核心数据。
    """
    rows = turso.query(
        """
        SELECT time_bucket,
               AVG(called_no_typical) AS called_typical,
               AVG(called_no_slow) AS called_slow,
               AVG(called_no_fast) AS called_fast,
               AVG(busy_rate) AS busy_rate,
               AVG(wait_typical_minutes) AS wait_minutes,
               AVG(queue_groups_typical) AS queue_groups,
               SUM(called_sample_count) AS samples
        FROM store_bucket_rollups
        WHERE store_id=? AND date_type=? AND time_bucket >= '10:00' AND time_bucket <= '22:00'
        GROUP BY time_bucket
        ORDER BY time_bucket
        """,
        (store_id, date_type),
    )
    name_row = turso.query("SELECT name FROM store_dimension WHERE store_id=?", (store_id,))
    name = name_row[0]["name"] if name_row else f"store {store_id}"
    points = []
    for r in rows:
        # 跳过完全无叫号且无排队的时段
        if r.get("called_typical") is None and (r.get("busy_rate") or 0) == 0:
            continue
        points.append({
            "time": r["time_bucket"],
            "called_typical": round(r["called_typical"]) if r.get("called_typical") is not None else None,
            "called_slow": round(r["called_slow"]) if r.get("called_slow") is not None else None,
            "called_fast": round(r["called_fast"]) if r.get("called_fast") is not None else None,
            "busy_rate": round(r.get("busy_rate") or 0, 2),
            "wait_minutes": round(r["wait_minutes"]) if r.get("wait_minutes") is not None else None,
            "queue_groups": round(r["queue_groups"]) if r.get("queue_groups") is not None else None,
            "samples": r.get("samples") or 0,
        })
    peak = max(points, key=lambda p: p.get("called_typical") or 0) if points else None
    return {
        "store_id": store_id,
        "store_name": name,
        "date_type": date_type,
        "buckets": points,
        "peak_bucket": {"time": peak["time"], "called_typical": peak["called_typical"]} if peak else None,
        "note": "各时段历史典型叫到几号（跨同类型各天平均）；busy_rate=该时段多大概率在排队；samples 少=数据待积累。"
        if points else "该店该日期类型暂无足够历史数据，请过几天再查。",
    }


def store_pressure(
    turso: TursoClient, store_id: int, date_type: str = "weekday"
) -> Dict[str, Any]:
    """某店各时段排队压力（忙率/等位/桌数），突出"几点最挤"。跨 weekday 平均。"""
    rows = turso.query(
        """
        SELECT time_bucket,
               AVG(busy_rate) AS busy_rate,
               AVG(wait_typical_minutes) AS wait_minutes,
               AVG(queue_groups_typical) AS queue_groups
        FROM store_bucket_rollups
        WHERE store_id=? AND date_type=? AND time_bucket >= '10:00' AND time_bucket <= '22:00'
        GROUP BY time_bucket
        ORDER BY time_bucket
        """,
        (store_id, date_type),
    )
    name_row = turso.query("SELECT name FROM store_dimension WHERE store_id=?", (store_id,))
    name = name_row[0]["name"] if name_row else f"store {store_id}"
    busy = [r for r in rows if (r.get("busy_rate") or 0) > 0]
    busiest = sorted(busy, key=lambda r: r.get("busy_rate") or 0, reverse=True)[:3]
    return {
        "store_id": store_id,
        "store_name": name,
        "date_type": date_type,
        "busiest_buckets": [
            {"time": r["time_bucket"], "busy_rate": round(r.get("busy_rate") or 0, 2),
             "wait_minutes": round(r["wait_minutes"]) if r.get("wait_minutes") is not None else None,
             "queue_groups": round(r["queue_groups"]) if r.get("queue_groups") is not None else None}
            for r in busiest
        ],
        "all_buckets": [
            {"time": r["time_bucket"], "busy_rate": round(r.get("busy_rate") or 0, 2),
             "wait_minutes": round(r["wait_minutes"]) if r.get("wait_minutes") is not None else None}
            for r in rows
        ],
    }


def called_speed(turso: TursoClient, store_id: int) -> Dict[str, Any]:
    """某店叫号速度/吞吐率（called_intervals_rollups），反映叫号快慢。"""
    rows = turso.query(
        """
        SELECT date_type, time_bucket, interval_typical_seconds, delta_typical,
               throughput_per_hour, capacity_utilization, pair_count
        FROM called_intervals_rollups
        WHERE store_id=? AND time_bucket >= '10:00' AND time_bucket <= '22:00'
        ORDER BY date_type, time_bucket
        """,
        (store_id,),
    )
    name_row = turso.query("SELECT name FROM store_dimension WHERE store_id=?", (store_id,))
    name = name_row[0]["name"] if name_row else f"store {store_id}"
    return {
        "store_id": store_id,
        "store_name": name,
        "speed_points": [
            {"date_type": r["date_type"], "time": r["time_bucket"],
             "interval_seconds": r.get("interval_typical_seconds"),
             "calls_per_call": r.get("delta_typical"),
             "throughput_per_hour": r.get("throughput_per_hour"),
             "capacity_utilization": round(r["capacity_utilization"], 2) if r.get("capacity_utilization") else None,
             "pairs": r.get("pair_count", 0)}
            for r in rows
        ],
        "note": "throughput_per_hour=每小时叫多少号；interval_seconds=两次叫号间隔；pairs 少=数据待积累。"
        if rows else "该店叫号速度数据待积累（需营业时段采集几天）。",
    }


def compare_stores(
    turso: TursoClient, store_ids: List[int], date_type: str = "weekday", time_bucket: Optional[str] = None
) -> List[Dict[str, Any]]:
    """多店对比某时段（或全天峰值）的叫号/忙率。"""
    if not store_ids:
        return []
    placeholders = ",".join("?" * len(store_ids))
    sql = f"""
        SELECT r.store_id, d.name, r.time_bucket, r.called_no_typical, r.busy_rate,
               r.wait_typical_minutes, r.queue_groups_typical
        FROM store_bucket_rollups r
        JOIN store_dimension d ON d.store_id = r.store_id
        WHERE r.store_id IN ({placeholders}) AND r.date_type=?
    """
    params = tuple(store_ids) + (date_type,)
    if time_bucket:
        sql += " AND r.time_bucket=?"
        params += (time_bucket,)
    sql += " ORDER BY r.store_id, r.time_bucket"
    rows = turso.query(sql, params)
    # 每店取叫号最高的桶
    by_store: Dict[int, Dict[str, Any]] = {}
    for r in rows:
        sid = r["store_id"]
        cur = by_store.get(sid)
        called = r.get("called_no_typical") or 0
        if cur is None or called > (cur.get("called_typical") or 0):
            by_store[sid] = {
                "store_id": sid,
                "store_name": r["name"],
                "time": r["time_bucket"],
                "called_typical": r.get("called_no_typical"),
                "busy_rate": round(r.get("busy_rate", 0), 2),
                "wait_minutes": r.get("wait_typical_minutes"),
                "queue_groups": r.get("queue_groups_typical"),
            }
    return list(by_store.values())
