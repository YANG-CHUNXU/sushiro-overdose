"""叫号算法。移植自 Go internal/app/queue_live.go CurrentCalledNo()。

寿司郎 getStoreById 返回 groupQueues，里面 4 个队列：
- boothQueue / mixedQueue / counterQueue：堂食桌位号（同一套号段）
- reservationQueue：预约号（独立号段，数值很大，不参与堂食叫号统计）

当前堂食叫到几号 = booth/mixed/counter 三队列里数值最大的整数。
返回 0 表示当前没有可用叫号（关店/无人排队）。
"""
from __future__ import annotations

from typing import Any, Dict, List, Optional


def _parse_queue_ints(queue: Any) -> List[int]:
    """把叫号字符串数组解析成 int 列表，跳过非数字。"""
    out: List[int] = []
    if not queue:
        return out
    if isinstance(queue, str):
        queue = [queue]
    if not isinstance(queue, list):
        return out
    for raw in queue:
        if raw is None:
            continue
        s = str(raw).strip()
        if not s:
            continue
        try:
            n = int(s)
            if n > 0:
                out.append(n)
        except ValueError:
            continue
    return out


def current_called_no(group_queues: Optional[Dict[str, Any]]) -> int:
    """返回堂食当前叫到的号。reservationQueue 不参与。0 = 无叫号。"""
    if not group_queues or not isinstance(group_queues, dict):
        return 0
    best = 0
    for key in ("boothQueue", "mixedQueue", "counterQueue"):
        for n in _parse_queue_ints(group_queues.get(key)):
            if n > best:
                best = n
    return best


def has_called_no(group_queues: Optional[Dict[str, Any]]) -> bool:
    """groupQueues 字段是否存在且包含至少一个堂食队列键（即使值为空也算"取到了"）。

    用于区分：接口返回了 groupQueues（哪怕没人在排）vs 接口根本没返回这个字段。
    """
    if not group_queues or not isinstance(group_queues, dict):
        return False
    return any(k in group_queues for k in ("boothQueue", "mixedQueue", "counterQueue"))
