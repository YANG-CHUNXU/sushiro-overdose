"""初始化节假日日历。

国内法定节假日/调休需要手动维护（官方每年公布）。这里提供：
- 内置近几年主要节假日（可扩展）
- 从 JSON 文件导入（holidays.json: {"holidays":[...], "workdays":[...]}）

节假日影响 date_type 判定，进而影响分桶（holiday/weekend/weekday/workday）。
不初始化也能跑，但所有日期都按 weekday/weekend 分，节假日模式会并错桶。
"""
from __future__ import annotations

import json
import logging
import os
from typing import List

from .turso import TursoClient

log = logging.getLogger("collector.holidays")

# 内置近几年国内主要法定节假日（date_key, name）。调休工作日单独标 workday。
# 来源：国务院公告。如需更全/更新，编辑后重新 seed-holidays。
BUILTIN_HOLIDAYS = [
    # 2025
    ("2025-01-01", "元旦"),
    ("2025-01-28", "春节"), ("2025-01-29", "春节"), ("2025-01-30", "春节"),
    ("2025-01-31", "春节"), ("2025-02-01", "春节"), ("2025-02-02", "春节"), ("2025-02-03", "春节"),
    ("2025-04-04", "清明"), ("2025-04-05", "清明"), ("2025-04-06", "清明"),
    ("2025-05-01", "劳动节"), ("2025-05-02", "劳动节"), ("2025-05-03", "劳动节"), ("2025-05-04", "劳动节"), ("2025-05-05", "劳动节"),
    ("2025-05-31", "端午"), ("2025-06-01", "端午"), ("2025-06-02", "端午"),
    ("2025-10-01", "国庆"), ("2025-10-02", "国庆"), ("2025-10-03", "国庆"),
    ("2025-10-04", "国庆"), ("2025-10-05", "国庆"), ("2025-10-06", "国庆"), ("2025-10-07", "国庆"),
    ("2025-10-08", "中秋"),
    # 2026
    ("2026-01-01", "元旦"), ("2026-01-02", "元旦"), ("2026-01-03", "元旦"),
    ("2026-02-15", "春节"), ("2026-02-16", "春节"), ("2026-02-17", "春节"),
    ("2026-02-18", "春节"), ("2026-02-19", "春节"), ("2026-02-20", "春节"), ("2026-02-21", "春节"),
    ("2026-04-04", "清明"), ("2026-04-05", "清明"), ("2026-04-06", "清明"),
    ("2026-05-01", "劳动节"), ("2026-05-02", "劳动节"), ("2026-05-03", "劳动节"), ("2026-05-04", "劳动节"), ("2026-05-05", "劳动节"),
    ("2026-06-19", "端午"), ("2026-06-20", "端午"), ("2026-06-21", "端午"),
    ("2026-09-25", "中秋"), ("2026-09-26", "中秋"), ("2026-09-27", "中秋"),
    ("2026-10-01", "国庆"), ("2026-10-02", "国庆"), ("2026-10-03", "国庆"),
    ("2026-10-04", "国庆"), ("2026-10-05", "国庆"), ("2026-10-06", "国庆"), ("2026-10-07", "国庆"),
]

# 调休工作日（周末补班）
BUILTIN_WORKDAYS = [
    "2025-01-26", "2025-02-08", "2025-04-27", "2025-09-28", "2025-10-11",
    "2026-02-14", "2026-02-22", "2026-04-26", "2026-09-27", "2026-10-10",
]


def seed_holidays(turso: TursoClient, year: int = 0) -> int:
    """把内置节假日写入 holiday_calendar。year>0 则只写该年。返回写入条数。"""
    entries: List[tuple] = []
    for date_key, name in BUILTIN_HOLIDAYS:
        if year and not date_key.startswith(str(year)):
            continue
        entries.append((date_key, "holiday", name, ""))
    for date_key in BUILTIN_WORKDAYS:
        if year and not date_key.startswith(str(year)):
            continue
        entries.append((date_key, "workday", "调休补班", ""))

    # 也尝试从 holidays.json 读（若存在，用户自定义）
    hj = os.path.join(os.getcwd(), "holidays.json")
    if os.path.exists(hj):
        with open(hj, "r", encoding="utf-8") as f:
            data = json.load(f)
        for k in data.get("holidays", []):
            entries.append((k, "holiday", "", ""))
        for k in data.get("workdays", []):
            entries.append((k, "workday", "", ""))

    if not entries:
        log.warning("没有节假日可写入")
        return 0

    sql = """
    INSERT INTO holiday_calendar (date_key, date_type, name, region)
    VALUES (?, ?, ?, ?)
    ON CONFLICT(date_key) DO UPDATE SET date_type=excluded.date_type, name=excluded.name
    """
    turso.execute_many([(sql, e) for e in entries])
    log.info("✅ 写入 %d 条节假日", len(entries))
    return len(entries)
