"""日期类型判定 + 时段桶。移植自 Go internal/app/queue_trends.go。

date_type 优先级：workday_override > holiday > 周末窗口 > weekday。
时段桶是 30 分钟（HH:00 / HH:30），与旧库 store_bucket_rollups.time_bucket 粒度一致。
"""
from __future__ import annotations

from datetime import datetime
from typing import Set, Tuple


def date_type_for(
    dt: datetime, holidays: Set[str], workdays: Set[str]
) -> Tuple[str, int]:
    """返回 (date_type, weekday)。weekday: Monday=0 ... Sunday=6（Python 约定，与旧库一致需确认）。"""
    key = dt.strftime("%Y-%m-%d")
    if key in workdays:
        dt_str = "workday"
    elif key in holidays:
        dt_str = "holiday"
    elif _weekend_window(dt):
        dt_str = "weekend"
    else:
        dt_str = "weekday"
    return dt_str, dt.weekday()


def _weekend_window(dt: datetime) -> bool:
    """周末窗口（移植 Go queueTrendWeekendWindow）。

    周五 16:30 及之后 / 整个周六 / 周日 22:00 之前 → 算周末行为模式。
    """
    seconds = dt.hour * 3600 + dt.minute * 60 + dt.second
    w = dt.weekday()  # Monday=0 ... Friday=4, Saturday=5, Sunday=6
    if w == 4:  # Friday
        return seconds >= 16 * 3600 + 30 * 60
    if w == 5:  # Saturday
        return True
    if w == 6:  # Sunday
        return seconds < 22 * 3600
    return False


def half_hour_bucket(dt: datetime) -> str:
    """30 分钟桶：'HH:00' 或 'HH:30'。与旧库 store_bucket_rollups.time_bucket 一致。"""
    minute = 30 if dt.minute >= 30 else 0
    return f"{dt.hour:02d}:{minute:02d}"


def snapshot_date(dt: datetime) -> str:
    return dt.strftime("%Y-%m-%d")


def parse_iso_cst(s: str) -> datetime:
    """解析 ISO8601 带时区字符串，转 CST。"""
    # Python 3.7+ datetime.fromisoformat 支持 +08:00 偏移
    dt = datetime.fromisoformat(s)
    if dt.tzinfo is None:
        from datetime import timezone, timedelta
        dt = dt.replace(tzinfo=timezone(timedelta(hours=8)))
    return dt
