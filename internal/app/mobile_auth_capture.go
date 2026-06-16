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

// mobileAuthCaptureTTL 是一次手机凭证捕获会话的最长存活时间：超时自动停止，避免代理端口和
// 引导页长期挂着。20 分钟覆盖"装证书+设代理+开微信点几个页面"的完整操作窗口。
const mobileAuthCaptureTTL = 20 * time.Minute

// mobileAuthCaptureManager 管理一次手机凭证捕获会话：本地起一个 MITM 代理抓手机真实请求，
// 同时起一个引导页（带随机 token 路径）给手机下证书、看代理地址。完成后把抓到的令牌落盘。
// 全程 mu 保护：watch 协程与 HTTP handler 会并发访问这些字段。
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
		writeError(w, http.StatusInternalServerError, "启动手机凭证捕获失败: "+err.Error())
		return
	}
	writeJSON(w, status)
}

func handleMobileAuthStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	mobileAuthCapture.stop("已停止手机凭证捕获")
	writeJSON(w, mobileAuthCapture.status())
}

// start 启动一次手机凭证捕获：先停掉旧会话，再用自签 CA 起 MITM 代理、起引导页 HTTP 服务。
// 安全/可用要点：①与主流程互斥（捕获时不能同时抢票，避免把手机会话顶掉）；
// ②引导页路径含随机 token（newMobileUAToken），防同网段他人乱扫；
// ③引导页监听 0.0.0.0:0（随机端口）——必须绑 0.0.0.0，手机才能通过电脑局域网 IP 访问到。
func (m *mobileAuthCaptureManager) start() (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked("")

	if status := engine.GetState().Status; status == EngineCapturing || status == EngineBooking || status == EngineSniping {
		return nil, fmt.Errorf("当前主流程正在运行（%s），请先停止后再启动手机凭证捕获", status)
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
	// 引导页路径带随机 token：不公开固定路径，必须拿到本次启动返回的 URL 才能访问，降低被同网段扫描到的风险。
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

	// 监听 0.0.0.0:0（随机端口）：必须绑 0.0.0.0 而不是 127.0.0.1，否则手机走电脑局域网 IP
	// 访问引导页时连不上；端口交给系统分配，避免固定端口被占。
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
	m.message = "手机凭证捕获已启动，请按引导页设置手机代理并打开寿司郎小程序。"
	m.logs = nil

	go func() {
		if err := guideServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			m.addLog("手机凭证引导页退出: " + err.Error())
		}
	}()
	go m.watch(ctx, token)
	return m.statusLocked(), nil
}

// watch 是捕获会话的后台循环：每秒轮询令牌是否抓全；到 TTL 强制超时停止；
// ctx 取消（用户主动 stop）则退出。注意 token 入参用于"本次启动"的身份比对——
// 若中途被新一次 start 替换（m.token 已变），本协程要自觉退出，避免误关新会话。
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
			m.stop("手机凭证捕获已超时，请重新启动")
			return
		case <-ticker.C:
			// sameRun 比对本次启动的 token：只有仍是自己这一次会话、且 tokens 还在，才检查完成。
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

// finish 在令牌抓全后落盘：先存 UA，再存全部凭证参数，然后刷新前端设置、补默认门店、
// 标记健康并停止会话。任何一步失败都记日志并改写 message 给用户，但不抛错中断流程。
// markAuthHealthy 是因为重新抓到有效凭证 → 此前"凭证过期"提醒应同步清除。
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
		m.addLog("保存手机凭证参数失败: " + err.Error())
		m.mu.Lock()
		m.message = "凭证参数已捕获，但保存失败: " + err.Error()
		m.mu.Unlock()
		return
	}
	markAuthHealthy() // 手机重新捕获凭证 → 清除"凭证过期"提醒
	setWebSettings(tokens.ToSettingsWithPrefs(prefs))
	tokens.Lock()
	if len(tokens.StoreIDs) > 0 && len(prefs.SelectedStores) == 0 {
		prefs.SelectedStores = tokens.StoreIDs
		SavePreferences(prefs)
	}
	tokens.Unlock()

	m.mu.Lock()
	m.saved = true
	m.message = "手机凭证参数已保存。请关闭手机 Wi-Fi 代理，再回电脑测试基础接口。"
	m.mu.Unlock()
	m.stop("手机凭证参数已保存")
}

func (m *mobileAuthCaptureManager) stop(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked(message)
}

// stopLocked 关闭本次会话的全部资源：取消 watch 协程的 ctx、关代理、优雅关引导页 HTTP 服务、
// 释放主流程占用标记（doneActivity），并清空运行态字段。message 非空时覆盖给用户的提示。
// 必须在持 mu 时调用（stop/start 内部都先锁）。
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

// addLog 同时写全局日志和本次会话的内存日志切片；切片封顶 120 条（丢最旧），
// 避免长时间会话累积过多日志拖累前端轮询。
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
<title>寿司郎手机凭证捕获</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,"PingFang SC",sans-serif;background:#f7f1ef;color:#1f1b18;margin:0;padding:24px;line-height:1.6}
.card{max-width:720px;margin:auto;background:#fff;border-radius:18px;padding:22px;box-shadow:0 12px 40px rgba(120,30,30,.12)}
h1{font-size:22px;margin:0 0 12px;color:#b81c22}.muted{color:#666}.warn{background:#fff6d8;border:1px solid #f2cf6a;border-radius:12px;padding:12px}
code{background:#f4f4f4;border-radius:6px;padding:2px 6px;word-break:break-all}.btn{display:inline-block;background:#b81c22;color:#fff;text-decoration:none;border-radius:999px;padding:10px 18px;font-weight:700}
li{margin:8px 0}
</style><div class="card">
<h1>寿司郎手机凭证捕获</h1>
<p class="muted">这一步让手机真实微信产生凭证请求，电脑只捕获寿司郎接口参数。完成后请关闭手机 Wi-Fi 代理。</p>
<p><a class="btn" href="` + html.EscapeString(data.CAURL) + `">下载并安装 CA 证书</a></p>
<div class="warn">本页面向 iPhone：安装描述文件后，还需要到“设置 → 通用 → 关于本机 → 证书信任设置”里完全信任该证书，否则抓不到 HTTPS。安卓请改用电脑上的“手动导入凭证”（用手机抓包工具导出请求再粘贴），不需要这一页。</div>
<h2>手机 Wi-Fi 代理</h2>
<p>把当前 Wi-Fi 的 HTTP 代理改为“手动”，服务器和端口填下面任意一个：</p>
<ul>` + hostList + `</ul>
<h2>操作步骤</h2>
<ol>
<li>保持本页不关，先安装并信任 CA。</li>
<li>回到 Wi-Fi 设置，手动代理指向上面的电脑 IP 和端口。</li>
<li>彻底关闭再打开手机微信，进入寿司郎小程序。</li>
<li>点一次门店列表、排队、预约或我的预约，让电脑捕获完整凭证参数。</li>
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
