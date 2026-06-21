"""联动桌面端 tools（调 127.0.0.1:39871 只读 GET，优雅降级）。

这些 tool 让 AI 查用户自己的状态：凭证是否有效、我的预约、手里排队号、环境诊断、可约时段。
桌面端没跑时返回友好提示。
"""
from __future__ import annotations

from typing import Any, Dict, Optional

from .desktop import DesktopClient


def desktop_status(desktop: DesktopClient) -> Dict[str, Any]:
    """桌面端总状态：版本/凭证健康(auth_health)/引擎状态/采样/通知配置。

    AI 应先调这个判断桌面端在不在跑、凭证有没有效。
    """
    data = desktop.get("/api/status")
    if not data.get("ok"):
        return data
    # 裁剪给 AI 的关键字段
    auth = data.get("auth_health") or {}
    engine = data.get("engine") or {}
    return {
        "ok": True,
        "running": data.get("running", False),
        "version": data.get("version", ""),
        "has_config": data.get("has_config", False),
        "auth_status": auth.get("status", "unknown"),  # ok / stale / unknown
        "auth_reason": auth.get("reason", ""),
        "engine_status": engine.get("status", "idle"),
        "engine_message": engine.get("message", ""),
        "notify_configured": data.get("notify_configured", False),
        "hint": "auth_status=stale 表示凭证过期需重新获取；ok 表示有效。" if data.get("has_config") else "尚未配置凭证。",
    }


def my_reservations(desktop: DesktopClient) -> Dict[str, Any]:
    """我的预约 + 排队号（需凭证）。"""
    data = desktop.get("/api/reservations")
    if not data.get("ok"):
        return data
    if data.get("unavailable"):
        return {"ok": False, "hint": data.get("message", "预约接口暂不可用，可能凭证失效"), "items": []}
    items = data.get("items") or data.get("reservations") or []
    return {
        "ok": True,
        "count": len(items),
        "items": [
            {
                "kind": it.get("kind", ""),
                "store": it.get("store_name") or it.get("store", ""),
                "date": it.get("date") or it.get("target_date", ""),
                "time": it.get("time") or it.get("slot_start", ""),
                "status": it.get("status", ""),
                "number": it.get("number"),  # 排队号
            }
            for it in items
        ],
    }


def my_ticket_status(desktop: DesktopClient) -> Dict[str, Any]:
    """手里排队号实时状态 + 取号计划。"""
    data = desktop.get("/api/queue/ticket/status")
    if not data.get("ok"):
        return data
    ticket = data.get("ticket") or {}
    plan = data.get("plan") or {}
    return {
        "ok": True,
        "has_ticket": bool(ticket and ticket.get("number")),
        "ticket": {
            "number": ticket.get("number"),
            "store": ticket.get("store_name") or ticket.get("store"),
            "status": ticket.get("status"),
            "date": ticket.get("date"),
        } if ticket else None,
        "plan": {
            "state": plan.get("state"),  # armed / idle / success / issued_unknown
            "store": plan.get("store_name") or plan.get("store"),
            "trigger_at": plan.get("trigger_at") or plan.get("target_time"),
        } if plan else None,
    }


def diagnose(desktop: DesktopClient) -> Dict[str, Any]:
    """全套环境诊断：凭证完整性/证书/代理链/网络。AI 排障利器。"""
    data = desktop.get("/api/diagnostics")
    if not data.get("ok"):
        return data
    cfg = data.get("config") or {}
    cert = data.get("certificate") or {}
    net = data.get("network") or {}
    proxy_marker = data.get("proxy_marker") or {}
    return {
        "ok": True,
        "config_complete": cfg.get("complete", False),
        "config_missing": cfg.get("missing", []),
        "cert_trusted": cert.get("trusted", False),
        "cert_error": cert.get("error", ""),
        "network_ok": net.get("ok", False),
        "network_latency_ms": net.get("latency_ms"),
        "proxy_residual": proxy_marker.get("stale", False),  # 残留代理设置
        "hint": "config_complete=False 缺凭证字段；cert_trusted=False 证书没装好；proxy_residual=True 有残留代理需修复。",
    }


def available_slots(
    desktop: DesktopClient, store: Optional[str] = None, period: Optional[str] = None
) -> Dict[str, Any]:
    """可约时段（需凭证 + 绑定门店）。period: lunch/dinner/all。"""
    params: Dict[str, Any] = {"available": 1}
    if store:
        params["store"] = store
    if period:
        params["period"] = period
    data = desktop.get("/api/calendar", params=params)
    if not data.get("ok"):
        return data
    slots = []
    for s in data.get("slots", []) or []:
        slots.append({
            "date": s.get("date"),
            "time": s.get("time") or s.get("slot_start"),
            "available": s.get("available", True),
        })
    return {
        "ok": True,
        "store": data.get("store_name", ""),
        "count": len(slots),
        "slots": slots[:20],  # 裁剪，避免太多
    }
