package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const mobileUACaptureTTL = 10 * time.Minute

type mobileUACaptureManager struct {
	mu        sync.Mutex
	server    *http.Server
	token     string
	url       string
	urls      []string
	qr        string
	startedAt time.Time
	expiresAt time.Time
	last      MobileUAConfig
}

var mobileUACapture = &mobileUACaptureManager{}

func handleMobileUA(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, mobileUACapture.status())
	case http.MethodPost, http.MethodPut:
		var req struct {
			UserAgent string `json:"user_agent"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
			return
		}
		cfg, err := SaveMobileUA(req.UserAgent, "manual", r.RemoteAddr)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "保存 UA 失败: "+err.Error())
			return
		}
		refreshWebClient()
		writeJSON(w, map[string]any{"ok": true, "config": cfg, "status": mobileUACapture.status()})
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST")
	}
}

func handleMobileUACaptureStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	status, err := mobileUACapture.start()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "启动 UA 采集失败: "+err.Error())
		return
	}
	writeJSON(w, status)
}

func handleMobileUACaptureStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	mobileUACapture.stop()
	writeJSON(w, mobileUACapture.status())
}

func (m *mobileUACaptureManager) start() (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()

	token := newMobileUAToken()
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	urls := mobileUACaptureURLs(port, token)
	primary := urls[0]
	mux := http.NewServeMux()
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	path := "/ua/" + token
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		cfg, err := SaveMobileUA(r.UserAgent(), "wechat-scan", r.RemoteAddr)
		if err == nil {
			m.mu.Lock()
			m.last = cfg
			m.mu.Unlock()
			refreshWebClient()
		}
		writeMobileUACapturePage(w, cfg, err)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<meta name="viewport" content="width=device-width,initial-scale=1"><body style="font-family:sans-serif;padding:24px"><h2>采集地址已失效</h2><p>请回到电脑端设置页重新启动扫码采集。</p></body>`))
	})

	now := time.Now()
	m.server = server
	m.token = token
	m.url = primary
	m.urls = urls
	m.qr = qrSVG(primary)
	m.startedAt = now
	m.expiresAt = now.Add(mobileUACaptureTTL)
	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			LogMessage(time.Now(), "UA 采集服务退出: "+err.Error())
		}
	}()
	go m.expire(token, mobileUACaptureTTL)
	return m.statusLocked(), nil
}

func (m *mobileUACaptureManager) stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

func (m *mobileUACaptureManager) stopLocked() {
	if m.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = m.server.Shutdown(ctx)
		cancel()
	}
	m.server = nil
	m.token = ""
	m.url = ""
	m.urls = nil
	m.qr = ""
	m.startedAt = time.Time{}
	m.expiresAt = time.Time{}
}

func (m *mobileUACaptureManager) expire(token string, after time.Duration) {
	time.Sleep(after)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token == token {
		m.stopLocked()
	}
}

func (m *mobileUACaptureManager) status() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *mobileUACaptureManager) statusLocked() map[string]any {
	status := map[string]any{
		"active":   m.server != nil,
		"url":      m.url,
		"urls":     m.urls,
		"qr_svg":   m.qr,
		"started":  m.startedAt,
		"expires":  m.expiresAt,
		"path":     MobileUAPath(),
		"ttl_secs": int(mobileUACaptureTTL.Seconds()),
	}
	if cfg, err := LoadMobileUA(); err == nil {
		status["config"] = cfg
	} else if m.last.NormalizedUserAgent != "" {
		status["config"] = m.last
	}
	return status
}

func newMobileUAToken() string {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func mobileUACaptureURLs(port int, token string) []string {
	ips := localIPv4s()
	if len(ips) == 0 {
		ips = []string{"127.0.0.1"}
	}
	urls := make([]string, 0, len(ips))
	for _, ip := range ips {
		urls = append(urls, fmt.Sprintf("http://%s:%d/ua/%s", ip, port, token))
	}
	return urls
}

func localIPv4s() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var ips []string
	seen := map[string]struct{}{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := addrIP(addr).To4()
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			text := ip.String()
			if _, ok := seen[text]; ok {
				continue
			}
			seen[text] = struct{}{}
			ips = append(ips, text)
		}
	}
	sort.SliceStable(ips, func(i, j int) bool {
		return privateIPScore(ips[i]) < privateIPScore(ips[j])
	})
	return ips
}

func addrIP(addr net.Addr) net.IP {
	switch a := addr.(type) {
	case *net.IPNet:
		return a.IP
	case *net.IPAddr:
		return a.IP
	default:
		return nil
	}
}

func privateIPScore(ip string) int {
	if strings.HasPrefix(ip, "192.168.") {
		return 0
	}
	if strings.HasPrefix(ip, "10.") {
		return 1
	}
	if strings.HasPrefix(ip, "172.") {
		parts := strings.Split(ip, ".")
		if len(parts) > 1 {
			second := 0
			_, _ = fmt.Sscanf(parts[1], "%d", &second)
			if second >= 16 && second <= 31 {
				return 2
			}
		}
	}
	return 3
}

func writeMobileUACapturePage(w http.ResponseWriter, cfg MobileUAConfig, err error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`<meta name="viewport" content="width=device-width,initial-scale=1"><body style="font-family:sans-serif;padding:24px"><h2>保存失败</h2><p>` + html.EscapeString(err.Error()) + `</p></body>`))
		return
	}
	raw := html.EscapeString(cfg.UserAgent)
	normalized := html.EscapeString(cfg.NormalizedUserAgent)
	_, _ = w.Write([]byte(`<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1"><title>UA 已采集</title><body style="font-family:-apple-system,BlinkMacSystemFont,'PingFang SC',sans-serif;background:#f7f2ef;margin:0;padding:24px;color:#151515"><main style="max-width:560px;margin:auto;background:#fff;border-radius:20px;padding:24px;box-shadow:0 16px 48px rgba(0,0,0,.08)"><h1 style="font-size:24px;margin:0 0 12px;color:#b81c22">UA 已采集</h1><p>可以回到电脑端设置页刷新状态，然后重新获取认证。</p><h2 style="font-size:16px;margin-top:20px">规范化 UA</h2><p style="word-break:break-all;background:#f5f5f5;padding:12px;border-radius:12px">` + normalized + `</p><details><summary>原始 UA</summary><p style="word-break:break-all;background:#f5f5f5;padding:12px;border-radius:12px">` + raw + `</p></details></main></body>`))
}
