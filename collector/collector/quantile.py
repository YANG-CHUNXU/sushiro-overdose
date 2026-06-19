"""分位数算法。移植自 Go internal/app/queue_trends.go queueQuantile（线性插值）。

排序 → pos = q*(n-1) → lo/hi 加权。空集返回 None。
"""
from __future__ import annotations

from typing import List, Optional


def quantile(values: List[float], q: float) -> Optional[float]:
    """线性插值分位数。空列表/None 返回 None。"""
    nums = [v for v in values if v is not None]
    n = len(nums)
    if n == 0:
        return None
    if n == 1:
        return float(nums[0])
    s = sorted(nums)
    pos = q * (n - 1)
    lo = int(pos)
    hi = min(lo + 1, n - 1)
    frac = pos - lo
    return s[lo] + (s[hi] - s[lo]) * frac


def rounded_quantile(values: List[float], q: float) -> Optional[float]:
    """分位数 + 四舍五入取整（None 时返回 None）。"""
    v = quantile(values, q)
    if v is None:
        return None
    return float(round(v))
