"""寿司郎公开接口客户端。

移植自 Go internal/app/queue_live.go。两个接口：
- stores?        批量列表，全国门店，无叫号（groupQueues）
- getStoreById?  单店，带叫号（groupQueues：booth/mixed/counter/reservation）

纯 urllib，无外部依赖。
"""
from __future__ import annotations

import json
import logging
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Dict, List, Optional

from .models import StoreInfo

log = logging.getLogger("collector.sushiro")

DEFAULT_BASE_URL = "https://crm-cn-prd.sushiro.com.cn/wechat/api/2.0"
DEFAULT_REFER = "https://servicewechat.com/wx7ac31ef6c073a7ed/159/page-frame.html"
DEFAULT_UA = "Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36 MicroMessenger/8.0"


class SushiroError(Exception):
    pass


class SushiroClient:
    def __init__(
        self,
        token: str,
        base_url: str = DEFAULT_BASE_URL,
        referer: str = DEFAULT_REFER,
        user_agent: str = DEFAULT_UA,
        timeout: float = 15.0,
    ):
        if not token:
            raise SushiroError("SushiroClient 需要 token")
        self.token = token
        self.base_url = base_url.rstrip("/")
        self.referer = referer
        self.user_agent = user_agent
        self.timeout = timeout

    def _get(self, path: str, query: Dict[str, Any]) -> Any:
        qs = urllib.parse.urlencode({k: v for k, v in query.items() if v is not None})
        url = f"{self.base_url}/{path.lstrip('/')}?{qs}"
        req = urllib.request.Request(
            url,
            method="GET",
            headers={
                "Authorization": f"Bearer {self.token}",
                "Referer": self.referer,
                "User-Agent": self.user_agent,
                "Accept": "*/*",
            },
        )
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                body = resp.read().decode("utf-8", "replace")
        except urllib.error.HTTPError as e:
            detail = e.read().decode("utf-8", "replace")[:300]
            raise SushiroError(f"HTTP {e.code} {path}: {detail}") from None
        except urllib.error.URLError as e:
            raise SushiroError(f"网络错误 {path}: {e}") from None
        try:
            return json.loads(body)
        except json.JSONDecodeError as e:
            raise SushiroError(f"响应非 JSON {path}: {body[:200]}") from e

    def list_stores(self, latitude: float = 23.13, longitude: float = 113.26) -> List[StoreInfo]:
        """批量列表接口。返回全国门店，无叫号。"""
        data = self._get(
            "stores",
            {"latitude": latitude, "longitude": longitude, "numresults": 10000},
        )
        arr = _extract_array(data)
        return [StoreInfo.from_api(d) for d in arr if isinstance(d, dict)]

    def get_store(self, store_id: int) -> Optional[StoreInfo]:
        """单店接口。带 groupQueues（营业+有人排队时）。返回 None 表示找不到。"""
        data = self._get("getStoreById", {"storeId": store_id})
        obj = data
        if isinstance(data, dict) and "storeId" not in data and "id" not in data:
            # 可能包在 data 字段里
            inner = data.get("data")
            if isinstance(inner, dict):
                obj = inner
        if not isinstance(obj, dict):
            return None
        return StoreInfo.from_api(obj)


def _extract_array(data: Any) -> List[Dict[str, Any]]:
    """接口可能返回裸数组 [...] 或 {key:[...]} 包装。"""
    if isinstance(data, list):
        return data
    if isinstance(data, dict):
        # 找第一个 list 值
        for v in data.values():
            if isinstance(v, list):
                return v
    return []
