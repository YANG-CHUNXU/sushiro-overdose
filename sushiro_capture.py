"""
mitmproxy addon: 从寿司郎微信小程序请求中自动提取预约所需参数
保存到 config_auto.json

使用方法:
1. 安装 mitmproxy: pip install mitmproxy
2. 手机配置代理指向电脑 IP:8080
3. 安装 mitmproxy CA 证书到手机（必须，否则 HTTPS 抓不到）
4. 运行: mitmdump -s sushiro_capture.py
5. 打开微信寿司郎小程序，随便浏览几个页面（门店列表、预约页面）
6. 参数会自动保存到 config_auto.json
"""

from __future__ import annotations

import json
import os
from datetime import datetime
from typing import Optional
from mitmproxy import http

CONFIG_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "config_auto.json")
SUSHIRO_HOST = "crm-cn-prd.sushiro.com.cn"

captured = {
    "x_app_code": None,
    "query_authorization": None,
    "reservation_authorization": None,
    "user_agent": None,
    "referer": None,
    "x_app_client": None,
    "store_ids": [],
    "phone_number": None,
    "wechat_id": None,
}


def save_captured():
    """保存已抓取的参数到 config_auto.json"""
    output = {}
    if captured["store_ids"]:
        output["store_ids"] = list(dict.fromkeys(captured["store_ids"]))
    if captured["phone_number"]:
        output["phone_number"] = captured["phone_number"]
    if captured["wechat_id"]:
        output["wechat_id"] = captured["wechat_id"]
    if captured["x_app_code"]:
        output["x_app_code"] = captured["x_app_code"]
    if captured["query_authorization"]:
        output["query_authorization"] = captured["query_authorization"]
    if captured["reservation_authorization"]:
        output["reservation_authorization"] = captured["reservation_authorization"]
    if captured["user_agent"]:
        output["user_agent"] = captured["user_agent"]
    if captured["referer"]:
        output["referer"] = captured["referer"]
    if captured["x_app_client"]:
        output["x_app_client"] = captured["x_app_client"]

    output["_captured_at"] = datetime.now().isoformat()
    output["_note"] = "此文件由 mitmproxy 自动生成，请检查后重命名为 config.json 使用"

    with open(CONFIG_PATH, "w", encoding="utf-8") as f:
        json.dump(output, f, ensure_ascii=False, indent=2)


def extract_store_id_from_url(path: str) -> Optional[str]:
    """从 URL 中提取 storeId 参数"""
    if "storeId=" in path:
        import urllib.parse
        params = urllib.parse.parse_qs(path.split("?", 1)[1] if "?" in path else "")
        ids = params.get("storeId", [])
        return ids[0] if ids else None
    return None


def response(flow: http.HTTPFlow) -> None:
    """拦截寿司郎 API 响应，提取参数"""
    if SUSHIRO_HOST not in flow.request.pretty_host:
        return

    req = flow.request
    headers = dict(req.headers)

    # 提取通用 headers
    if "X-App-Code" in headers and not captured["x_app_code"]:
        captured["x_app_code"] = headers["X-App-Code"]
        print(f"[CAPTURED] x_app_code = {captured['x_app_code']}")

    if "User-Agent" in headers and not captured["user_agent"]:
        captured["user_agent"] = headers["User-Agent"]
        print(f"[CAPTURED] user_agent = {captured['user_agent']}")

    if "Referer" in headers and not captured["referer"]:
        captured["referer"] = headers["Referer"]
        print(f"[CAPTURED] referer = {captured['referer']}")

    if "X-App-Client" in headers and not captured["x_app_client"]:
        captured["x_app_client"] = headers["X-App-Client"]
        print(f"[CAPTURED] x_app_client = {captured['x_app_client']}")

    # 提取 Authorization（区分查询和预约）
    auth_header = headers.get("Authorization", "")
    if auth_header:
        path = req.path
        if "/api_auth/" in path or "createReservation" in path:
            if not captured["reservation_authorization"]:
                captured["reservation_authorization"] = auth_header
                print(f"[CAPTURED] reservation_authorization = {auth_header[:50]}...")
        else:
            if not captured["query_authorization"]:
                captured["query_authorization"] = auth_header
                print(f"[CAPTURED] query_authorization = {auth_header[:50]}...")

    # 提取 storeId
    store_id = extract_store_id_from_url(req.path)
    if store_id and store_id not in captured["store_ids"]:
        captured["store_ids"].append(store_id)
        print(f"[CAPTURED] store_id = {store_id}")

    # 提取请求体中的 wechatId 和 phoneNumber
    if req.method == "POST" and req.content:
        try:
            body = json.loads(req.content)
            if "wechatId" in body and not captured["wechat_id"]:
                captured["wechat_id"] = body["wechatId"]
                print(f"[CAPTURED] wechat_id = {captured['wechat_id']}")
            if "phoneNumber" in body and not captured["phone_number"]:
                captured["phone_number"] = body["phoneNumber"]
                print(f"[CAPTURED] phone_number = {captured['phone_number']}")
        except (json.JSONDecodeError, UnicodeDecodeError):
            pass

    # 每次都保存
    save_captured()
