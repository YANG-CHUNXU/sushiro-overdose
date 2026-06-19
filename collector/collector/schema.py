"""新 Turso 库 schema（DDL）。

设计原则：压力 + 叫号同表分层（queue_snapshots），用 dq_source + display_called_no IS NULL
区分"没取叫号"(NULL) vs "取了但当前无堂食叫号"(0)。表名/列名兼容现有 Cloudflare Worker
和 Go 客户端（store_dimension/store_latest/store_bucket_rollups + called_no_*），桌面端零改动。
"""
from __future__ import annotations

from typing import List

# 按顺序执行。CREATE INDEX 跟在对应表后面。
SCHEMA_STATEMENTS: List[str] = [
    # 1. 门店维度
    """
    CREATE TABLE IF NOT EXISTS store_dimension (
      store_id           INTEGER PRIMARY KEY,
      name               TEXT NOT NULL,
      city               TEXT, area TEXT, address TEXT,
      latitude           REAL, longitude REAL,
      open_date          TEXT,
      tables_capacity    INTEGER, counters_capacity INTEGER,
      first_seen_at      TEXT, last_seen_at TEXT,
      is_active          INTEGER NOT NULL DEFAULT 1
    )
    """,
    # 2. 每店最新一帧（worker 查它做实时总览；叫号列兼容 worker 两段式查询）
    """
    CREATE TABLE IF NOT EXISTS store_latest (
      store_id           INTEGER PRIMARY KEY,
      collected_at       TEXT NOT NULL,
      name TEXT, city TEXT, area TEXT,
      wait_minutes       INTEGER NOT NULL DEFAULT 0,
      group_queues_count INTEGER NOT NULL DEFAULT 0,
      store_status TEXT, net_ticket_status TEXT, reservation_status TEXT,
      online_open        INTEGER NOT NULL DEFAULT 0,
      wait_time_counter  INTEGER, wait_time_cap INTEGER,
      display_called_no  INTEGER,
      group_queues_json  TEXT,
      updated_at         TEXT NOT NULL
    )
    """,
    # 3. 原始逐帧快照（压力+叫号同表，聚合源）
    """
    CREATE TABLE IF NOT EXISTS queue_snapshots (
      id                 INTEGER PRIMARY KEY AUTOINCREMENT,
      collected_at       TEXT NOT NULL,
      store_id           INTEGER NOT NULL,
      wait_minutes       INTEGER NOT NULL DEFAULT 0,
      group_queues_count INTEGER NOT NULL DEFAULT 0,
      store_status TEXT, net_ticket_status TEXT, reservation_status TEXT,
      online_open        INTEGER NOT NULL DEFAULT 0,
      wait_time_counter  INTEGER, wait_time_cap INTEGER,
      display_called_no  INTEGER,
      group_queues_json  TEXT,
      dq_source          TEXT NOT NULL,
      api_profile_version TEXT,
      dq_anomaly         INTEGER NOT NULL DEFAULT 0
    )
    """,
    "CREATE INDEX IF NOT EXISTS idx_snap_store_time ON queue_snapshots(store_id, collected_at)",
    "CREATE INDEX IF NOT EXISTS idx_snap_collected  ON queue_snapshots(collected_at)",
    # 幂等去重：同 collected_at+store_id+dq_source 唯一，让 migrate 可安全重跑（INSERT OR IGNORE 跳过已存在）
    "CREATE UNIQUE INDEX IF NOT EXISTS ux_snap_dedup ON queue_snapshots(collected_at, store_id, dq_source)",
    # 4. 时段聚合（worker 查它画热力图/趋势/到店建议；含叫号三档）
    """
    CREATE TABLE IF NOT EXISTS store_bucket_rollups (
      store_id           INTEGER NOT NULL,
      date_type          TEXT NOT NULL,
      weekday            INTEGER NOT NULL,
      time_bucket        TEXT NOT NULL,
      sample_count       INTEGER NOT NULL DEFAULT 0,
      open_rate          REAL, online_open_rate REAL, busy_rate REAL,
      wait_typical_minutes REAL,
      wait_safe_minutes  REAL,
      wait_max_minutes   INTEGER,
      queue_groups_typical REAL,
      queue_groups_safe  REAL,
      called_sample_count INTEGER NOT NULL DEFAULT 0,
      called_no_slow     REAL,
      called_no_typical  REAL,
      called_no_fast     REAL,
      dq_anomaly_rate    REAL,
      confidence         TEXT,
      updated_at         TEXT NOT NULL,
      PRIMARY KEY (store_id, date_type, weekday, time_bucket)
    )
    """,
    "CREATE INDEX IF NOT EXISTS idx_rollup_store ON store_bucket_rollups(store_id)",
    # 5. 按天细分 rollup
    """
    CREATE TABLE IF NOT EXISTS daily_store_bucket_rollups (
      snapshot_date      TEXT NOT NULL,
      store_id           INTEGER NOT NULL,
      date_type TEXT, weekday INTEGER, time_bucket TEXT,
      sample_count INTEGER, open_count INTEGER, online_open_count INTEGER, busy_count INTEGER,
      open_rate REAL, online_open_rate REAL, busy_rate REAL,
      wait_typical_minutes REAL, wait_safe_minutes REAL, wait_max_minutes INTEGER,
      queue_groups_typical REAL, queue_groups_safe REAL,
      called_sample_count INTEGER, called_no_slow REAL, called_no_typical REAL, called_no_fast REAL,
      confidence TEXT, updated_at TEXT NOT NULL,
      PRIMARY KEY (snapshot_date, store_id, time_bucket)
    )
    """,
    # 6. 叫号间隔/吞吐率聚合（预测辅助）
    """
    CREATE TABLE IF NOT EXISTS called_intervals_rollups (
      store_id           INTEGER NOT NULL,
      date_type          TEXT NOT NULL,
      time_bucket        TEXT NOT NULL,
      pair_count         INTEGER NOT NULL,
      interval_typical_seconds REAL,
      delta_typical      REAL,
      throughput_per_hour REAL,
      capacity_utilization REAL,
      updated_at         TEXT NOT NULL,
      PRIMARY KEY (store_id, date_type, time_bucket)
    )
    """,
    # 7. 节假日日历
    """
    CREATE TABLE IF NOT EXISTS holiday_calendar (
      date_key           TEXT PRIMARY KEY,
      date_type          TEXT NOT NULL,
      name               TEXT,
      region             TEXT
    )
    """,
    # 8. 采集任务日志
    """
    CREATE TABLE IF NOT EXISTS collector_runs (
      run_id             TEXT PRIMARY KEY,
      started_at TEXT, finished_at TEXT,
      endpoint TEXT, stores_seen INTEGER, records_written INTEGER,
      ok INTEGER, error_message TEXT, api_profile_version TEXT
    )
    """,
    # 9. 归档日志
    """
    CREATE TABLE IF NOT EXISTS archive_runs (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      archive_date TEXT, archive_path TEXT,
      snapshots INTEGER, rollups_written INTEGER, global_rollups INTEGER,
      raw_deleted INTEGER, raw_remaining INTEGER, retention_days INTEGER,
      prune_before TEXT, ok INTEGER, error_message TEXT,
      created_at TEXT, updated_at TEXT
    )
    """,
]
