CREATE TABLE IF NOT EXISTS queue_stats (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  schema_version INTEGER NOT NULL,
  client_version TEXT NOT NULL,
  install_id_hash TEXT NOT NULL,
  store_id TEXT NOT NULL,
  weekday INTEGER NOT NULL,
  time_bucket TEXT NOT NULL,
  table_type TEXT NOT NULL,
  party_size_bucket TEXT NOT NULL,
  samples INTEGER NOT NULL,
  wait_p50_minutes REAL,
  wait_p80_minutes REAL,
  checkin_to_call_p50_minutes REAL,
  missed_rate REAL NOT NULL,
  received_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX IF NOT EXISTS idx_queue_stats_lookup
ON queue_stats (store_id, weekday, time_bucket, table_type, party_size_bucket);

CREATE INDEX IF NOT EXISTS idx_queue_stats_received_at
ON queue_stats (received_at);
