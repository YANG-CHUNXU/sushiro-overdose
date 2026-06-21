"""Turso 只读客户端（libsql-python，DB-API 2.0 风格）。

启动时建一次连接，全程复用（MCP 是长驻子进程）。
只暴露具体语义查询函数，绝不暴露任意 SQL。
"""
from __future__ import annotations

import logging
from typing import Any, Dict, List, Optional

import libsql

log = logging.getLogger("mcp.turso")


class TursoClient:
    def __init__(self, url: str, token: str):
        if not url or not token:
            raise ValueError("TursoClient 需要 url 和 token")
        # libsql.connect(url, auth_token=...)；远程库必须带 token
        self._url = url
        self._token = token
        self._conn = None
        self._connect()

    def _connect(self) -> None:
        try:
            # libsql-python：connect("libsql://...", auth_token="...")
            self._conn = libsql.connect(self._url, auth_token=self._token)
            log.info("Turso 连接成功: %s", self._url.split("@")[-1] if "@" in self._url else self._url)
        except Exception as e:
            log.error("Turso 连接失败: %s", e)
            raise

    def query(self, sql: str, params: Optional[tuple] = None) -> List[Dict[str, Any]]:
        """执行只读 SELECT，返回 [{col: value}, ...]。失败抛异常（上层捕获降级）。"""
        if self._conn is None:
            self._connect()
        cur = self._conn.cursor()
        if params:
            cur.execute(sql, params)
        else:
            cur.execute(sql)
        rows = cur.fetchall()
        cols = [d[0] for d in cur.description] if cur.description else []
        return [dict(zip(cols, r)) for r in rows]

    def close(self) -> None:
        if self._conn:
            try:
                self._conn.close()
            except Exception:
                pass
            self._conn = None
