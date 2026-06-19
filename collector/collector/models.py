"""数据模型（dataclass）。对应寿司郎接口返回 + Turso 快照行。"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, Optional


@dataclass
class StoreInfo:
    """寿司郎接口返回的单店信息（stores? 和 getStoreById? 共有字段的超集）。"""

    store_id: int
    name: str = ""
    name_kana: str = ""
    city: str = ""
    area: str = ""
    address: str = ""
    latitude: Optional[float] = None
    longitude: Optional[float] = None
    distance: str = ""
    store_status: str = ""          # OPEN / CLOSED
    net_ticket_status: str = ""     # ONLINE / OFFLINE_*
    wait_minutes: int = 0
    wait_time_counter: Optional[int] = None
    wait_time_cap: Optional[int] = None
    tables_capacity: int = 0
    counters_capacity: int = 0
    group_queues_count: int = 0
    reservation_status: str = ""
    open_date: str = ""
    # group_queues 仅 getStoreById 返回
    group_queues: Optional[Dict[str, Any]] = None

    @property
    def online_open(self) -> bool:
        s = (self.net_ticket_status or "").upper()
        if "CLOSED" in s or "OFFLINE" in s:
            return False
        if s in ("ONLINE", "ON") or "OPEN" in s:
            return True
        return False

    @classmethod
    def from_api(cls, d: Dict[str, Any]) -> "StoreInfo":
        """从接口 JSON（驼峰键）构造。兼容 id / storeId 两种主键名。"""
        sid = d.get("storeId")
        if sid is None:
            sid = d.get("id")
        try:
            store_id = int(sid) if sid is not None else 0
        except (TypeError, ValueError):
            store_id = 0

        lat = d.get("latitude")
        lng = d.get("longitude")

        def _int(key: str) -> int:
            v = d.get(key)
            try:
                return int(v) if v is not None else 0
            except (TypeError, ValueError):
                return 0

        def _oint(key: str) -> Optional[int]:
            v = d.get(key)
            if v is None or v == "":
                return None
            try:
                return int(v)
            except (TypeError, ValueError):
                return None

        return cls(
            store_id=store_id,
            name=str(d.get("name") or ""),
            name_kana=str(d.get("nameKana") or ""),
            city=str(d.get("city") or ""),
            area=str(d.get("area") or ""),
            address=str(d.get("address") or ""),
            latitude=float(lat) if lat not in (None, "") else None,
            longitude=float(lng) if lng not in (None, "") else None,
            distance=str(d.get("distance") or ""),
            store_status=str(d.get("storeStatus") or ""),
            net_ticket_status=str(d.get("netTicketStatus") or ""),
            wait_minutes=_int("wait"),
            wait_time_counter=_oint("waitTimeCounter"),
            wait_time_cap=_oint("waitTimeCap"),
            tables_capacity=_int("tablesCapacity"),
            counters_capacity=_int("countersCapacity"),
            group_queues_count=_int("groupQueuesCount"),
            reservation_status=str(d.get("reservationStatus") or ""),
            open_date=str(d.get("openDate") or ""),
            group_queues=d.get("groupQueues") if isinstance(d.get("groupQueues"), dict) else None,
        )


@dataclass
class Snapshot:
    """一行 queue_snapshots：单店单帧。display_called_no=None 表示本轮没取叫号。"""

    collected_at: str
    store_id: int
    wait_minutes: int = 0
    group_queues_count: int = 0
    store_status: str = ""
    net_ticket_status: str = ""
    reservation_status: str = ""
    online_open: int = 0
    wait_time_counter: Optional[int] = None
    wait_time_cap: Optional[int] = None
    display_called_no: Optional[int] = None  # None=没取叫号; 0=取了但无堂食叫号; >0=当前叫到
    group_queues_json: Optional[str] = None
    dq_source: str = "stores_list"           # 'stores_list' | 'store_detail'
    api_profile_version: str = ""
    dq_anomaly: int = 0

    @classmethod
    def from_store(
        cls,
        store: StoreInfo,
        collected_at: str,
        dq_source: str,
        include_called: bool,
        api_profile_version: str = "public-profile-v1",
    ) -> "Snapshot":
        import json as _json

        gq_json = None
        called_no: Optional[int] = None
        if include_called:
            # 取了叫号：把 group_queues 序列化；叫号值用算法算（可能 0）
            gq = store.group_queues or {}
            gq_json = _json.dumps(gq, ensure_ascii=False) if gq else None
            from .called_no import current_called_no
            called_no = current_called_no(gq)

        return cls(
            collected_at=collected_at,
            store_id=store.store_id,
            wait_minutes=store.wait_minutes,
            group_queues_count=store.group_queues_count,
            store_status=store.store_status,
            net_ticket_status=store.net_ticket_status,
            reservation_status=store.reservation_status,
            online_open=1 if store.online_open else 0,
            wait_time_counter=store.wait_time_counter,
            wait_time_cap=store.wait_time_cap,
            display_called_no=called_no,
            group_queues_json=gq_json,
            dq_source=dq_source,
            api_profile_version=api_profile_version,
        )
