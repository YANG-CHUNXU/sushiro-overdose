package app

import (
	"time"

	. "github.com/Ryujoxys/sushiro-overdose/internal/core"
)

// cmdMCPDaemon 是 --mcp-daemon-child 子命令：开机自启时由系统拉起。
// 职责：确保 MCP venv 就绪 + 写 claude_desktop_config.json，然后退出（不常驻——
// MCP 进程由 Claude Desktop 通过 claude_desktop_config 自己拉起）。
func cmdMCPDaemon() {
	LogMessage(time.Now(), "MCP daemon child: 准备 MCP 环境")
	cfg := LoadMCPConfig()
	if !cfg.Enabled {
		LogMessage(time.Now(), "MCP daemon child: MCP 未启用，跳过")
		return
	}
	mcpDir := findMCPDir()
	if mcpDir == "" {
		LogMessage(time.Now(), "MCP daemon child: 找不到 mcp/ 目录，跳过")
		return
	}
	if err := EnsureMCPVenv(mcpDir); err != nil {
		LogMessage(time.Now(), "MCP daemon child: venv 准备失败: "+err.Error())
		return
	}
	if err := registerClaudeDesktop(mcpDir, cfg); err != nil {
		LogMessage(time.Now(), "MCP daemon child: 写 Claude 配置失败: "+err.Error())
		return
	}
	LogMessage(time.Now(), "MCP daemon child: 完成（venv 就绪，Claude 配置已写）")
}
