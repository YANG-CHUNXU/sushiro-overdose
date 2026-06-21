"""配置加载：环境变量 > .env > 默认。

Turso token / 桌面端端口从环境变量读（桌面端拉起 MCP 时注入，或 .env 手填）。
"""
from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Optional


@dataclass
class Config:
    turso_url: str
    turso_token: str
    desktop_port: int

    @property
    def turso_configured(self) -> bool:
        return bool(self.turso_url and self.turso_token)


def _load_env_file(path: str = ".env") -> None:
    """简易 .env 加载（不依赖 python-dotenv）：KEY=VALUE 逐行注入 os.environ（不覆盖已有）。"""
    try:
        with open(path, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                k, _, v = line.partition("=")
                k, v = k.strip(), v.strip()
                if k and k not in os.environ:
                    os.environ[k] = v
    except FileNotFoundError:
        pass


def load_config() -> Config:
    _load_env_file()
    port_str = os.environ.get("SUSHIRO_MCP_DESKTOP_PORT", "39871")
    try:
        port = int(port_str)
    except ValueError:
        port = 39871
    return Config(
        turso_url=os.environ.get("SUSHIRO_MCP_TURSO_URL", ""),
        turso_token=os.environ.get("SUSHIRO_MCP_TURSO_TOKEN", ""),
        desktop_port=port,
    )
