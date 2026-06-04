package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"
import . "github.com/Ryujoxys/sushiro-overdose/internal/proxy"

import (
	"context"
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const mobileAuthCaptureTTL = 20 * time.Minute

type mobileAuthCaptureManager struct {
	mu           sync.Mutex
	proxy        *ProxyServer
	guideServer  *http.Server
	tokens       *CapturedTokens
	cancel       context.CancelFunc
	doneActivity func()
	token        string
	proxyPort    int
	guidePort    int
	hosts        []string
	urls         []string
	qr           string
	startedAt    time.Time
	expiresAt    time.Time
	saved        bool
	message      string
	logs         []LogEntry
}

var mobileAuthCapture = &mobileAuthCaptureManager{}

func handleMobileAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	writeJSON(w, mobileAuthCapture.status())
}

func handleMobileAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	status, err := mobileAuthCapture.start()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "启动手机认证捕获失败: "+err.Error())
		return
	}
	writeJSON(w, status)
}

func handleMobileAuthStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	mobileAuthCapture.stop("已停止手机认证捕获")
	writeJSON(w, mobileAuthCapture.status())
}

func (m *mobileAuthCaptureManager) start() (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked("")

	if status := engine.GetState().Status; status == EngineCapturing || status == EngineBooking || status == EngineSniping {
		return nil, fmt.Errorf("当前主流程正在运行（%s），请先停止后再启动手机认证捕获", status)
	}
	doneActivity := markMainFlowActive("mobile-auth-capturing")

	caCert, caKey, err := LoadOrGenerateCA()
	if err != nil {
		doneActivity()
		return nil, err
	}

	tokens := NewCapturedTokens()
	proxy, err := StartMobileCaptureProxy(caCert, caKey, tokens, m.addLog)
	if err != nil {
		doneActivity()
		return nil, err
	}

	token := newMobileUAToken()
	hosts := localIPv4s()
	if len(hosts) == 0 {
		hosts = []string{"127.0.0.1"}
	}
	guideMux := http.NewServeMux()
	guideServer := &http.Server{
		Handler:           guideMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	pathPrefix := "/mobile-auth/" + token
	guideMux.HandleFunc(pathPrefix, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathPrefix {
			http.NotFound(w, r)
			return
		}
		writeMobileAuthGuide(w, mobileAuthGuideData{
			Hosts:     hosts,
			ProxyPort: proxy.Port(),
			CAURL:     pathPrefix + "/ca.crt",
		})
	})
	guideMux.HandleFunc(pathPrefix+"/ca.crt", func(w http.ResponseWriter, r *http.Request) {
		serveMobileAuthCA(w)
	})
	guideMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		proxy.Close()
		doneActivity()
		return nil, err
	}
	guidePort := ln.Addr().(*net.TCPAddr).Port
	urls := mobileAuthGuideURLs(hosts, guidePort, token)
	now := time.Now()
	ctx, cancel := context.WithCancel(context.Background())

	m.proxy = proxy
	m.guideServer = guideServer
	m.tokens = tokens
	m.cancel = cancel
	m.doneActivity = doneActivity
	m.token = token
	m.proxyPort = proxy.Port()
	m.guidePort = guidePort
	m.hosts = hosts
	m.urls = urls
	m.qr = qrSVG(urls[0])
	m.startedAt = now
	m.expiresAt = now.Add(mobileAuthCaptureTTL)
	m.saved = false
	m.message = "手机认证捕获已启动，请按引导页设置手机代理并打开寿司郎小程序。"
	m.logs = nil

	go func() {
		if err := guideServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			m.addLog("手机认证引导页退出: " + err.Error())
		}
	}()
	go m.watch(ctx, token)
	return m.statusLocked(), nil
}

func (m *mobileAuthCaptureManager) watch(ctx context.Context, token string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	timeout := time.NewTimer(mobileAuthCaptureTTL)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout.C:
			m.stop("手机认证捕获已超时，请重新启动")
			return
		case <-ticker.C:
			m.mu.Lock()
			sameRun := m.token == token && m.tokens != nil
			tokens := m.tokens
			m.mu.Unlock()
			if !sameRun {
				return
			}
			if tokens.IsComplete() {
				m.finish(tokens)
				return
			}
		}
	}
}

func (m *mobileAuthCaptureManager) finish(tokens *CapturedTokens) {
	prefs := LoadPreferences()
	tokens.Lock()
	rawUA := tokens.UserAgent
	tokens.Unlock()
	if strings.TrimSpace(rawUA) != "" {
		if _, err := SaveMobileUA(rawUA, "mobile-auth", "phone-proxy"); err != nil {
			m.addLog("保存手机 UA 失败: " + err.Error())
		}
	}
	if err := SaveLocalConfig(tokens); err != nil {
		m.addLog("保存手机认证参数失败: " + err.Error())
		m.mu.Lock()
		m.message = "认证参数已捕获，但保存失败: " + err.Error()
		m.mu.Unlock()
		return
	}
	setWebSettings(tokens.ToSettingsWithPrefs(prefs))
	tokens.Lock()
	if len(tokens.StoreIDs) > 0 && len(prefs.SelectedStores) == 0 {
		prefs.SelectedStores = tokens.StoreIDs
		SavePreferences(prefs)
	}
	tokens.Unlock()

	m.mu.Lock()
	m.saved = true
	m.message = "手机认证参数已保存。请关闭手机 Wi-Fi 代理，再回电脑测试基础接口。"
	m.mu.Unlock()
	m.stop("手机认证参数已保存")
}

func (m *mobileAuthCaptureManager) stop(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked(message)
}

func (m *mobileAuthCaptureManager) stopLocked(message string) {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if m.proxy != nil {
		m.proxy.Close()
		m.proxy = nil
	}
	if m.guideServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = m.guideServer.Shutdown(ctx)
		cancel()
		m.guideServer = nil
	}
	if m.doneActivity != nil {
		m.doneActivity()
		m.doneActivity = nil
	}
	m.token = ""
	m.proxyPort = 0
	m.guidePort = 0
	m.hosts = nil
	m.urls = nil
	m.qr = ""
	m.startedAt = time.Time{}
	m.expiresAt = time.Time{}
	if message != "" {
		m.message = message
	}
}

func (m *mobileAuthCaptureManager) status() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *mobileAuthCaptureManager) statusLocked() map[string]any {
	out := map[string]any{
		"active":     m.proxy != nil,
		"saved":      m.saved,
		"message":    m.message,
		"hosts":      m.hosts,
		"proxy_port": m.proxyPort,
		"guide_urls": m.urls,
		"qr_svg":     m.qr,
		"started":    m.startedAt,
		"expires":    m.expiresAt,
		"ttl_secs":   int(mobileAuthCaptureTTL.Seconds()),
		"ca_path":    filepath.Join(CertDirPath(), "ca.crt"),
		"logs":       append([]LogEntry(nil), m.logs...),
	}
	if m.tokens != nil {
		out["capture"] = captureStatusForTokens(m.tokens)
	}
	if tokens, err := LoadLocalConfig(); err == nil {
		out["config_complete"] = tokens.ValidateForReservation() == nil
	}
	return out
}

func (m *mobileAuthCaptureManager) addLog(msg string) {
	LogMessage(time.Now(), msg)
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := LogEntry{
		Time:    time.Now().Format("15:04:05"),
		Message: msg,
	}
	m.logs = append(m.logs, entry)
	if len(m.logs) > 120 {
		m.logs = m.logs[len(m.logs)-120:]
	}
}

func mobileAuthGuideURLs(hosts []string, port int, token string) []string {
	urls := make([]string, 0, len(hosts))
	for _, host := range hosts {
		urls = append(urls, fmt.Sprintf("http://%s:%d/mobile-auth/%s", host, port, token))
	}
	return urls
}

type mobileAuthGuideData struct {
	Hosts     []string
	ProxyPort int
	CAURL     string
}

func writeMobileAuthGuide(w http.ResponseWriter, data mobileAuthGuideData) {
	hostList := ""
	for _, host := range data.Hosts {
		hostList += "<li><code>" + html.EscapeString(host) + ":" + html.EscapeString(fmt.Sprint(data.ProxyPort)) + "</code></li>"
	}
	if hostList == "" {
		hostList = "<li><code>电脑IP:" + html.EscapeString(fmt.Sprint(data.ProxyPort)) + "</code></li>"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(`<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1">
<title>寿司郎手机认证捕获</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,"PingFang SC",sans-serif;background:#f7f1ef;color:#1f1b18;margin:0;padding:24px;line-height:1.6}
.card{max-width:720px;margin:auto;background:#fff;border-radius:18px;padding:22px;box-shadow:0 12px 40px rgba(120,30,30,.12)}
h1{font-size:22px;margin:0 0 12px;color:#b81c22}.muted{color:#666}.warn{background:#fff6d8;border:1px solid #f2cf6a;border-radius:12px;padding:12px}
code{background:#f4f4f4;border-radius:6px;padding:2px 6px;word-break:break-all}.btn{display:inline-block;background:#b81c22;color:#fff;text-decoration:none;border-radius:999px;padding:10px 18px;font-weight:700}
li{margin:8px 0}
</style><div class="card">
<h1>寿司郎手机认证捕获</h1>
<p class="muted">这一步让手机真实微信产生认证请求，电脑只捕获寿司郎接口参数。完成后请关闭手机 Wi-Fi 代理。</p>
<p><a class="btn" href="` + html.EscapeString(data.CAURL) + `">下载并安装 CA 证书</a></p>
<div class="warn">本页面向 iPhone：安装描述文件后，还需要到“设置 → 通用 → 关于本机 → 证书信任设置”里完全信任该证书，否则抓不到 HTTPS。安卓请改用电脑上的“手动导入认证”（用手机抓包工具导出请求再粘贴），不需要这一页。</div>
<h2>手机 Wi-Fi 代理</h2>
<p>把当前 Wi-Fi 的 HTTP 代理改为“手动”，服务器和端口填下面任意一个：</p>
<ul>` + hostList + `</ul>
<h2>操作步骤</h2>
<ol>
<li>保持本页不关，先安装并信任 CA。</li>
<li>回到 Wi-Fi 设置，手动代理指向上面的电脑 IP 和端口。</li>
<li>彻底关闭再打开手机微信，进入寿司郎小程序。</li>
<li>点一次门店列表、排队、预约或我的预约，让电脑捕获完整认证参数。</li>
<li>电脑提示捕获完成后，立刻把手机 Wi-Fi 代理改回“关闭”。</li>
</ol>
<p class="muted">如果设置代理后手机完全没网，通常是电脑防火墙或手机/电脑不在同一个局域网。</p>
</div>`))
}

func serveMobileAuthCA(w http.ResponseWriter) {
	certPath := filepath.Join(CertDirPath(), "ca.crt")
	data, err := os.ReadFile(certPath)
	if err != nil {
		http.Error(w, "CA certificate not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", `attachment; filename="sushiro-proxy-ca.crt"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}
