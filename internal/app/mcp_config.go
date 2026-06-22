package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/Ryujoxys/sushiro-overdose/internal/core"
)

// MCPConfig 是桌面端 MCP 助手的配置：是否启用、是否自启、Turso 只读连接。
// 存 ~/.sushiro/mcp_config.json，0600（含 Turso token，敏感，跟 config.json/notify.json 同级）。
type MCPConfig struct {
	Enabled    bool   `json:"enabled"`
	AutoStart  bool   `json:"auto_start"`
	TursoURL   string `json:"turso_url"`   // libsql://... 只读库地址
	TursoToken string `json:"turso_token"` // 只读 token（a:ro）
}

var (
	mcpConfigMu sync.Mutex
)

// MCPConfigPath 返回 MCP 配置落盘路径。
func MCPConfigPath() string {
	return filepath.Join(AppDirPath(), "mcp_config.json")
}

// DefaultMCPConfig 返回默认（空）MCP 配置。
func DefaultMCPConfig() MCPConfig {
	return MCPConfig{
		TursoURL: "libsql://su-shiro-ryujoxys.aws-us-west-2.turso.io",
	}
}

// NormalizeMCPConfig 规整：去空白、默认 Turso URL。
func NormalizeMCPConfig(cfg MCPConfig) MCPConfig {
	cfg.TursoURL = strings.TrimSpace(cfg.TursoURL)
	cfg.TursoToken = strings.TrimSpace(cfg.TursoToken)
	if cfg.TursoURL == "" {
		cfg.TursoURL = DefaultMCPConfig().TursoURL
	}
	// token 缺失时不能算 enabled（查数据 tool 用不了，但联动桌面端 tool 仍可用，故不强求）
	return cfg
}

// LoadMCPConfig 读 MCP 配置，失败/不存在返回默认。
func LoadMCPConfig() MCPConfig {
	mcpConfigMu.Lock()
	defer mcpConfigMu.Unlock()
	data, err := os.ReadFile(MCPConfigPath())
	if err != nil {
		return DefaultMCPConfig()
	}
	var cfg MCPConfig
	if json.Unmarshal(data, &cfg) != nil {
		return DefaultMCPConfig()
	}
	return NormalizeMCPConfig(cfg)
}

// SaveMCPConfig 原子写 MCP 配置（0600）。
func SaveMCPConfig(cfg MCPConfig) error {
	mcpConfigMu.Lock()
	defer mcpConfigMu.Unlock()
	cfg = NormalizeMCPConfig(cfg)
	os.MkdirAll(AppDirPath(), 0o755)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(MCPConfigPath(), data, 0o600)
}

// MCPConfigured 报告 Turso 是否配齐（URL+token）。
func (c MCPConfig) TursoConfigured() bool {
	return c.TursoURL != "" && c.TursoToken != ""
}
