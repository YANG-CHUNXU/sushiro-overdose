package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/platform"

import (
	"encoding/json"
	"net/http"
)

// handleMCP 处理 /api/mcp：GET 查状态，POST 改配置（enabled/auto_start/turso_*）。
// 开启时准备 venv + 写 claude_desktop_config.json（首次装依赖可能耗时，前端应给 loading）。
func handleMCP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, MCPStatus())
	case http.MethodPost, http.MethodPut:
		var req struct {
			Enabled    *bool   `json:"enabled,omitempty"`
			AutoStart  *bool   `json:"auto_start,omitempty"`
			TursoURL   *string `json:"turso_url,omitempty"`
			TursoToken *string `json:"turso_token,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
			return
		}
		cfg := LoadMCPConfig()
		if req.Enabled != nil {
			cfg.Enabled = *req.Enabled
		}
		if req.AutoStart != nil {
			cfg.AutoStart = *req.AutoStart
		}
		if req.TursoURL != nil {
			cfg.TursoURL = *req.TursoURL
		}
		if req.TursoToken != nil {
			cfg.TursoToken = *req.TursoToken
		}
		// 启用前必须配齐 Turso（否则 MCP 查数据 tool 用不了，但联动桌面端仍可用——这里不强制，
		// 允许先开再填 token，但提示用户）。
		if err := SaveMCPConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, "保存失败: "+err.Error())
			return
		}
		// 启停动作：开启则准备 venv + 注册；关闭则取消注册。
		if cfg.Enabled {
			if err := MCPEnable(cfg); err != nil {
				// venv 准备/注册失败：配置已存，但未生效。返回错误让前端提示，不回滚配置（用户可修后重试）。
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		} else {
			MCPDisable()
		}
		writeJSON(w, map[string]any{"ok": true, "status": MCPStatus()})
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST")
	}
}

// handleMCPAutostart 处理 /api/mcp/autostart：查/设开机自启。
func handleMCPAutostart(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, MCPAutoStartStatus())
	case http.MethodPost, http.MethodPut:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
			return
		}
		cfg := LoadMCPConfig()
		cfg.AutoStart = req.Enabled
		if err := SaveMCPConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, "保存失败: "+err.Error())
			return
		}
		if req.Enabled {
			if err := InstallMCPAutoStart(); err != nil {
				writeError(w, http.StatusInternalServerError, "设置自启失败: "+err.Error())
				return
			}
		} else {
			if err := RemoveMCPAutoStart(); err != nil {
				writeError(w, http.StatusInternalServerError, "取消自启失败: "+err.Error())
				return
			}
		}
		writeJSON(w, MCPAutoStartStatus())
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST")
	}
}
