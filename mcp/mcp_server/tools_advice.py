"""智能到店建议 tool（综合实时 ETA + 历史规律）。

最有价值的 tool：综合桌面端实时预测 + Turso 历史数据，给"几点取号/几点出发/这个时段挤不挤"的建议。
"""
from __future__ import annotations

from typing import Any, Dict, Optional

from .desktop import DesktopClient
from .turso import TursoClient


def arrival_advice(
    turso: TursoClient,
    desktop: DesktopClient,
    store_id: int,
    target_no: Optional[int] = None,
    want_meal_time: Optional[str] = None,
    travel_minutes: Optional[int] = None,
) -> Dict[str, Any]:
    """综合建议：实时 ETA + 历史规律。

    - target_no 给了：算"这个号大概几点叫到、几点出发"（调桌面端 advisor）
    - want_meal_time 给了（如 "18:30"）：算"想这个点吃，几点取号"（调桌面端 plan）
    - 叠加 Turso 历史忙率："这个时段挤不挤、叫号快慢"
    """
    advice: Dict[str, Any] = {"store_id": store_id}

    # 1. 实时预测（桌面端）
    realtime: Dict[str, Any] = {}
    if target_no:
        params = {"store": str(store_id), "target_no": str(target_no)}
        if travel_minutes:
            params["travel_minutes"] = str(travel_minutes)
        d = desktop.get("/api/queue/advisor", params)
        if d.get("ok"):
            eta = d.get("eta") or {}
            current = d.get("current") or {}
            pressure = d.get("pressure") or {}
            realtime = {
                "called_no": current.get("called_no"),
                "waiting_groups": current.get("waiting_groups"),
                "eta_range": eta.get("estimated_called_at_range"),
                "wait_minutes_range": eta.get("wait_minutes_range"),
                "arrival_suggestion": eta.get("arrival_suggestion"),
                "pressure_level": pressure.get("label"),
                "source": eta.get("source_label"),
            }
        else:
            realtime = {"desktop_hint": d.get("hint", "桌面端不可用")}
    elif want_meal_time:
        params = {"target_meal": want_meal_time, "store": str(store_id)}
        if travel_minutes:
            params["travel_minutes"] = str(travel_minutes)
        d = desktop.get("/api/queue/plan", params)
        if d.get("ok"):
            realtime = {
                "want_meal_time": want_meal_time,
                "recommend_pickup_range": d.get("recommend_pickup_range"),
                "latest_pickup": d.get("latest_pickup"),
            }
        else:
            realtime = {"desktop_hint": d.get("hint", "桌面端不可用")}
    advice["realtime"] = realtime

    # 2. 历史规律（Turso）：该店忙率 + 叫号曲线摘要
    history: Dict[str, Any] = {}
    try:
        # 取最相关的 date_type（简单判断：周末/节假日优先看 weekend/holiday，否则 weekday）
        for dt in ("weekend", "holiday", "weekday"):
            rows = turso.query(
                """SELECT time_bucket, busy_rate, called_no_typical, wait_typical_minutes
                   FROM store_bucket_rollups
                   WHERE store_id=? AND date_type=? AND time_bucket >= '10:00' AND time_bucket <= '22:00'
                   AND (called_no_typical IS NOT NULL OR busy_rate > 0)
                   GROUP BY time_bucket ORDER BY time_bucket LIMIT 1""",
                (store_id, dt),
            )
            if rows:
                history["date_type"] = dt
                break
        if history.get("date_type"):
            dt = history["date_type"]
            # 该时段（want_meal_time 所在半小时桶）的历史
            if want_meal_time:
                bucket = _to_bucket(want_meal_time)
                hrows = turso.query(
                    """SELECT busy_rate, called_no_typical, wait_typical_minutes
                       FROM store_bucket_rollups WHERE store_id=? AND date_type=? AND time_bucket=?
                       GROUP BY time_bucket""",
                    (store_id, dt, bucket),
                )
                if hrows:
                    r = hrows[0]
                    history["target_bucket"] = bucket
                    history["busy_rate"] = round(r.get("busy_rate") or 0, 2)
                    history["historical_wait"] = r.get("wait_typical_minutes")
            # 峰值时段
            peak_rows = turso.query(
                """SELECT time_bucket, MAX(busy_rate) AS br FROM store_bucket_rollups
                   WHERE store_id=? AND date_type=? AND time_bucket >= '10:00' AND time_bucket <= '22:00'
                   GROUP BY time_bucket ORDER BY br DESC LIMIT 1""",
                (store_id, dt),
            )
            if peak_rows:
                history["peak_time"] = peak_rows[0]["time_bucket"]
                history["peak_busy_rate"] = round(peak_rows[0].get("br") or 0, 2)
    except Exception as e:
        history["error"] = str(e)
    advice["history"] = history

    # 3. 综合建议（人话）
    advice["summary"] = _summarize(realtime, history, target_no, want_meal_time)
    return advice


def _to_bucket(hhmm: str) -> str:
    """'18:30' → '18:30'；'18:45' → '18:30'（向下取整到半小时桶）。"""
    try:
        h, m = hhmm.split(":")
        m = "00" if int(m) < 30 else "30"
        return f"{int(h):02d}:{m}"
    except Exception:
        return hhmm


def _summarize(realtime, history, target_no, want_meal_time) -> str:
    parts = []
    if target_no and realtime.get("arrival_suggestion"):
        parts.append(realtime["arrival_suggestion"])
    if want_meal_time and realtime.get("recommend_pickup_range"):
        rng = realtime["recommend_pickup_range"]
        if isinstance(rng, dict):
            parts.append(f"想吃 {want_meal_time}，建议 {rng.get('early','?')}~{rng.get('late','?')} 之间取号")
        else:
            parts.append(f"想吃 {want_meal_time}，建议取号时段 {rng}")
    if history.get("busy_rate") is not None:
        br = history["busy_rate"]
        lvl = "很挤" if br >= 0.8 else ("较挤" if br >= 0.4 else "不挤")
        parts.append(f"该时段历史{lvl}（忙率{br:.0%}）")
    if history.get("peak_time"):
        parts.append(f"该店历史最挤在 {history['peak_time']} 左右，尽量避开")
    return "；".join(parts) + "。" if parts else "数据不足，建议多观察几天再定。"
