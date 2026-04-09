package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const proxyPort = 8080
const sushiroHost = "crm-cn-prd.sushiro.com.cn"

// TargetSlot represents a timeslot the user wants to book.
type TargetSlot struct {
	StoreID string
	Date    string // compact YYYYMMDD
	Start   string // compact HHMMSS
	End     string // compact HHMMSS
}

// CapturedTokens holds auth parameters intercepted from WeChat mini-program traffic.
type CapturedTokens struct {
	mu              sync.Mutex
	XAppCode        string
	QueryAuth       string
	ReservationAuth string
	UserAgent       string
	Referer         string
	XAppClient      string
	WechatID        string
	PhoneNumber     string
	StoreIDs        []string
	FeishuWebhook   string
}

func newCapturedTokens() *CapturedTokens {
	return &CapturedTokens{}
}

func (t *CapturedTokens) IsComplete() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.XAppCode != "" &&
		t.QueryAuth != "" &&
		t.ReservationAuth != "" &&
		t.UserAgent != "" &&
		t.Referer != "" &&
		t.WechatID != ""
}

func (t *CapturedTokens) Status() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	check := func(v string) string {
		if v != "" {
			return "✅"
		}
		return "⏳"
	}
	return []string{
		fmt.Sprintf("  X-App-Code:       %s %s", check(t.XAppCode), maskToken(t.XAppCode)),
		fmt.Sprintf("  Query Auth:       %s %s", check(t.QueryAuth), maskToken(t.QueryAuth)),
		fmt.Sprintf("  Reservation Auth: %s %s", check(t.ReservationAuth), maskToken(t.ReservationAuth)),
		fmt.Sprintf("  User-Agent:       %s %s", check(t.UserAgent), maskToken(t.UserAgent)),
		fmt.Sprintf("  Referer:          %s %s", check(t.Referer), maskToken(t.Referer)),
		fmt.Sprintf("  Wechat ID:        %s %s", check(t.WechatID), maskToken(t.WechatID)),
		fmt.Sprintf("  Phone Number:     %s %s", check(t.PhoneNumber), t.PhoneNumber),
		fmt.Sprintf("  Store IDs:        %s %v", check(fmt.Sprintf("%v", t.StoreIDs)), t.StoreIDs),
	}
}

func maskToken(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 12 {
		return "***"
	}
	return v[:8] + "..."
}

const localConfigFile = ".sushiro_local.json"

type localConfigJSON struct {
	XAppCode        string   `json:"x_app_code"`
	QueryAuth       string   `json:"query_authorization"`
	ReservationAuth string   `json:"reservation_authorization"`
	UserAgent       string   `json:"user_agent"`
	Referer         string   `json:"referer"`
	XAppClient      string   `json:"x_app_client"`
	WechatID        string   `json:"wechat_id"`
	PhoneNumber     string   `json:"phone_number"`
	StoreIDs        []string `json:"store_ids"`
}

func saveLocalConfig(tokens *CapturedTokens) error {
	tokens.mu.Lock()
	defer tokens.mu.Unlock()
	data := localConfigJSON{
		XAppCode:        tokens.XAppCode,
		QueryAuth:       tokens.QueryAuth,
		ReservationAuth: tokens.ReservationAuth,
		UserAgent:       tokens.UserAgent,
		Referer:         tokens.Referer,
		XAppClient:      tokens.XAppClient,
		WechatID:        tokens.WechatID,
		PhoneNumber:     tokens.PhoneNumber,
		StoreIDs:        tokens.StoreIDs,
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(localConfigFile, raw, 0o600)
}

func loadLocalConfig() (*CapturedTokens, error) {
	raw, err := os.ReadFile(localConfigFile)
	if err != nil {
		return nil, err
	}
	var data localConfigJSON
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data.XAppCode == "" || data.QueryAuth == "" {
		return nil, fmt.Errorf("config incomplete")
	}
	return &CapturedTokens{
		XAppCode:        data.XAppCode,
		QueryAuth:       data.QueryAuth,
		ReservationAuth: data.ReservationAuth,
		UserAgent:       data.UserAgent,
		Referer:         data.Referer,
		XAppClient:      data.XAppClient,
		WechatID:        data.WechatID,
		PhoneNumber:     data.PhoneNumber,
		StoreIDs:        data.StoreIDs,
		FeishuWebhook:   loadFeishuConfig(),
	}, nil
}

func deleteLocalConfig() {
	os.Remove(localConfigFile)
}

// Feishu webhook is stored separately so it survives token refreshes.
func feishuConfigPath() string {
	return filepath.Join(appDirPath(), "feishu.json")
}

func loadFeishuConfig() string {
	data, err := os.ReadFile(feishuConfigPath())
	if err != nil {
		return ""
	}
	var cfg struct {
		Webhook string `json:"webhook"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return ""
	}
	return strings.TrimSpace(cfg.Webhook)
}

func saveFeishuConfig(webhook string) {
	os.MkdirAll(appDirPath(), 0o755)
	cfg := struct {
		Webhook string `json:"webhook"`
	}{Webhook: strings.TrimSpace(webhook)}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(feishuConfigPath(), data, 0o644)
}

func (t *CapturedTokens) toSettings() Settings {
	t.mu.Lock()
	defer t.mu.Unlock()

	timezone := "Asia/Shanghai"
	location, _ := time.LoadLocation(timezone)

	return Settings{
		StoreIDs:           t.StoreIDs,
		Adult:              2,
		Child:              0,
		TableType:          "T",
		Debug:              true,
		PhoneNumber:        t.PhoneNumber,
		WechatID:           t.WechatID,
		XAppCode:           t.XAppCode,
		QueryAuthorization: t.QueryAuth,
		ReservationAuth:    t.ReservationAuth,
		XAppClient:         fallbackString(t.XAppClient, "miniapp"),
		UserAgent:          t.UserAgent,
		Referer:            t.Referer,
		StateFile:          ".sushiro_state.json",
		Timezone:           timezone,
		Location:           location,
		PollInterval:       60 * time.Second,
		AvailableStatuses:  map[string]struct{}{"AVAILABLE": {}},
		BaseURL:            "https://crm-cn-prd.sushiro.com.cn",
		FeishuWebhook:      t.FeishuWebhook,
	}
}

func (t *CapturedTokens) captureFromRequest(req *http.Request, bodyBytes []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	host := req.URL.Host
	if host == "" {
		host = req.Host
	}
	if !strings.Contains(host, sushiroHost) {
		return
	}

	if v := req.Header.Get("X-App-Code"); v != "" && t.XAppCode == "" {
		t.XAppCode = v
	}
	if v := req.Header.Get("User-Agent"); v != "" && t.UserAgent == "" {
		t.UserAgent = v
	}
	if v := req.Header.Get("Referer"); v != "" && t.Referer == "" {
		t.Referer = v
	}
	if v := req.Header.Get("X-App-Client"); v != "" && t.XAppClient == "" {
		t.XAppClient = v
	}

	authHeader := req.Header.Get("Authorization")
	if authHeader != "" {
		path := req.URL.Path
		if strings.Contains(path, "/api_auth/") || strings.Contains(path, "createReservation") {
			if t.ReservationAuth == "" {
				t.ReservationAuth = authHeader
			}
		} else {
			if t.QueryAuth == "" {
				t.QueryAuth = authHeader
			}
		}
	}

	if sid := req.URL.Query().Get("storeId"); sid != "" {
		found := false
		for _, existing := range t.StoreIDs {
			if existing == sid {
				found = true
				break
			}
		}
		if !found {
			t.StoreIDs = append(t.StoreIDs, sid)
		}
	}

	// Parse POST body for wechatId and phoneNumber
	if req.Method == http.MethodPost && len(bodyBytes) > 0 {
		var body map[string]any
		if json.Unmarshal(bodyBytes, &body) == nil {
			if wid, ok := body["wechatId"].(string); ok && t.WechatID == "" {
				t.WechatID = wid
			}
			if pn, ok := body["phoneNumber"].(string); ok && t.PhoneNumber == "" {
				t.PhoneNumber = pn
			}
		}
	}
}

// ---- MITM Proxy Server ----

type proxyServer struct {
	listener net.Listener
	done     chan struct{}
	caCert   tls.Certificate
	caKey    *rsa.PrivateKey
	tokens   *CapturedTokens
}

func startProxy(caCert tls.Certificate, caKey *rsa.PrivateKey, tokens *CapturedTokens) (*proxyServer, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort))
	if err != nil {
		return nil, fmt.Errorf("listen on %d: %w", proxyPort, err)
	}

	ps := &proxyServer{
		listener: listener,
		done:     make(chan struct{}),
		caCert:   caCert,
		caKey:    caKey,
		tokens:   tokens,
	}

	go ps.serve()
	return ps, nil
}

func (ps *proxyServer) close() {
	if ps.listener != nil {
		ps.listener.Close()
	}
	select {
	case <-ps.done:
	default:
		close(ps.done)
	}
}

func (ps *proxyServer) serve() {
	for {
		conn, err := ps.listener.Accept()
		if err != nil {
			select {
			case <-ps.done:
				return
			default:
				continue
			}
		}
		go ps.handleConn(conn)
	}
}

func (ps *proxyServer) handleConn(clientConn net.Conn) {
	defer clientConn.Close()

	br := bufio.NewReader(clientConn)

	// Read the request line
	firstLine, err := br.ReadString('\n')
	if err != nil {
		return
	}
	firstLine = strings.TrimSpace(firstLine)

	if !strings.HasPrefix(firstLine, "CONNECT ") {
		// Not a CONNECT — respond with error and move on
		fmt.Fprintf(clientConn, "HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\n")
		return
	}

	// Parse target host
	parts := strings.SplitN(firstLine, " ", 3)
	if len(parts) < 2 {
		return
	}
	hostPort := parts[1]

	// Drain remaining CONNECT headers
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	isSushiro := strings.Contains(hostPort, sushiroHost)

	// Dial the real server
	serverConn, err := net.DialTimeout("tcp", hostPort, 10*time.Second)
	if err != nil {
		fmt.Fprintf(clientConn, "HTTP/1.1 502 Bad Gateway\r\nConnection: close\r\n\r\n")
		return
	}
	defer serverConn.Close()

	// Tell client the tunnel is ready
	fmt.Fprintf(clientConn, "HTTP/1.1 200 Connection Established\r\n\r\n")

	if !isSushiro {
		// Flush any buffered data from br that read ahead past the CONNECT headers
		// Only needed for plain tunnel; MITM path does its own TLS handshake
		if br.Buffered() > 0 {
			buf := make([]byte, br.Buffered())
			n, _ := br.Read(buf)
			if n > 0 {
				serverConn.Write(buf[:n])
			}
		}
		// Plain tunnel — just relay bytes both ways
		go func() {
			io.Copy(serverConn, clientConn)
			serverConn.Close()
		}()
		io.Copy(clientConn, serverConn)
		return
	}

	// ---- MITM for sushiro ----

	// TLS handshake with real server
	serverName := hostPort
	if idx := strings.LastIndex(hostPort, ":"); idx != -1 {
		serverName = hostPort[:idx]
	}

	tlsServer := tls.Client(serverConn, &tls.Config{
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,
	})
	defer tlsServer.Close()
	if err := tlsServer.Handshake(); err != nil {
		logMessage(time.Now(), fmt.Sprintf("TLS handshake to %s failed: %v", serverName, err))
		return
	}

	// TLS handshake with client (using forged cert)
	hostCert, err := generateHostCert(ps.caCert, ps.caKey, serverName)
	if err != nil {
		return
	}
	tlsClient := tls.Server(clientConn, &tls.Config{
		Certificates: []tls.Certificate{hostCert},
		MinVersion:   tls.VersionTLS12,
	})
	defer tlsClient.Close()
	if err := tlsClient.Handshake(); err != nil {
		logMessage(time.Now(), fmt.Sprintf("TLS handshake from client failed: %v", err))
		return
	}

	logMessage(time.Now(), fmt.Sprintf("MITM established for %s", serverName))

	// Direct transport — bypass system proxy to avoid infinite loop
	directTransport := &http.Transport{
		TLSClientConfig: &tls.Config{},
		Proxy:           func(*http.Request) (*url.URL, error) { return nil, nil },
	}
	defer directTransport.CloseIdleConnections()

	// Read-Forward-Relay loop
	clientReader := bufio.NewReader(tlsClient)
	for {
		req, err := http.ReadRequest(clientReader)
		if err != nil {
			return // client closed connection
		}

		// Read body bytes
		var bodyBytes []byte
		if req.Body != nil {
			bodyBytes, _ = io.ReadAll(req.Body)
			req.Body.Close()
		}

		// Capture tokens
		ps.tokens.captureFromRequest(req, bodyBytes)

		// Rebuild request for forwarding
		req.URL.Scheme = "https"
		req.URL.Host = hostPort
		req.RequestURI = ""
		req.Header.Del("Proxy-Connection")
		req.Header.Del("Proxy-Authenticate")
		req.Header.Del("Proxy-Authorization")
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))

		// Forward
		resp, err := directTransport.RoundTrip(req)
		if err != nil {
			return
		}

		// Relay response
		resp.Write(tlsClient)
		resp.Body.Close()
	}
}

// ---- Capture wait loop ----

func waitForCapture(ctx context.Context, tokens *CapturedTokens, skip <-chan struct{}) error {
	fmt.Println("等待捕获认证参数...")
	fmt.Println("请在 PC 微信中打开寿司郎小程序")
	fmt.Println("⚠️ 需要在目标门店进行一次排队取号/预约，才能捕获全部参数")
	fmt.Println("按回车跳过等待（手动模式）...")
	fmt.Println()

	var lastStatus string
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-skip:
			return nil
		case <-ticker.C:
			currentStatus := strings.Join(tokens.Status(), "\n")
			if currentStatus != lastStatus {
				lastStatus = currentStatus
				for _, line := range tokens.Status() {
					fmt.Println(line)
				}
				fmt.Println()
			}

			if tokens.IsComplete() {
				fmt.Println("所有必要参数已捕获!")
				return nil
			}
		}
	}
}

// ---- Interactive selection ----

func selectStores(ctx context.Context, client *Client, tokens *CapturedTokens) ([]string, error) {
	tokens.mu.Lock()
	storeIDs := make([]string, len(tokens.StoreIDs))
	copy(storeIDs, tokens.StoreIDs)
	tokens.mu.Unlock()

	if len(storeIDs) == 0 {
		fmt.Print("未捕获到门店ID，请手动输入门店编号: ")
		return []string{readInput()}, nil
	}

	fmt.Println("\n--- 可选门店 ---")
	reg := GetStoreRegistry()
	for i, storeID := range storeIDs {
		storeInfo, err := client.GetStoreInfo(ctx, storeID)
		if err != nil {
			fmt.Printf("  %d. 门店 %s（获取详情失败: %v）\n", i+1, storeID, err)
			continue
		}
		displayName := reg.DisplayName(storeID, storeInfo.Name)
		nicknameTag := ""
		if displayName != storeInfo.Name {
			nicknameTag = fmt.Sprintf(" [%s]", displayName)
		}
		fmt.Printf("  %d. %s%s（%s）- %s\n", i+1, storeInfo.Name, nicknameTag, storeID, storeInfo.Address)
	}

	fmt.Print("\n请选择门店编号（多个用逗号分隔，直接回车选全部）: ")
	input := readInput()

	if input == "" {
		return storeIDs, nil
	}

	var selected []string
	for _, s := range strings.Split(input, ",") {
		s = strings.TrimSpace(s)
		var idx int
		if _, err := fmt.Sscanf(s, "%d", &idx); err == nil && idx >= 1 && idx <= len(storeIDs) {
			selected = append(selected, storeIDs[idx-1])
		}
	}
	if len(selected) == 0 {
		return storeIDs, nil
	}
	return selected, nil
}



// SlotPref defines what time range to target for a day type.
type SlotPref int

const (
	PrefNone      SlotPref = iota // 不预约
	Pref1930to2030                // 19:30-20:30
	PrefBefore2000                // 20:00前
	Pref1030to1300                // 10:30-13:00
)

var prefNames = map[SlotPref]string{
	PrefNone:      "不预约",
	Pref1930to2030: "19:30-20:30",
	PrefBefore2000: "20:00前",
	Pref1030to1300: "10:30-13:00",
}

// SlotConfig holds per-day-type slot preferences.
type SlotConfig struct {
	Weekday SlotPref
	Saturday SlotPref
	Sunday   SlotPref
}

func (c SlotConfig) shouldTarget(slot Slot, loc *time.Location) bool {
	day, err := parseCompactDate(slot.Date, loc)
	if err != nil {
		return false
	}
	weekday := day.Weekday()

	var pref SlotPref
	switch weekday {
	case time.Saturday:
		pref = c.Saturday
	case time.Sunday:
		pref = c.Sunday
	default:
		pref = c.Weekday
	}

	if pref == PrefNone {
		return false
	}

	start := slot.Start
	end := slot.End

	switch pref {
	case Pref1930to2030:
		return start >= "193000" && start < "203000" && end <= "203000"
	case PrefBefore2000:
		return start < "200000" && end <= "200000"
	case Pref1030to1300:
		return start >= "103000" && start < "130000" && end <= "130000"
	}
	return false
}

func configureSlots() SlotConfig {
	opts := []SlotPref{Pref1930to2030, PrefBefore2000, Pref1030to1300, PrefNone}

	choose := func(label string) SlotPref {
		for {
			fmt.Printf("\n%s:\n", label)
			for i, p := range opts {
				fmt.Printf("  %d. %s\n", i+1, prefNames[p])
			}
			fmt.Print("请选择: ")
			input := readInput()
			var idx int
			if _, err := fmt.Sscanf(input, "%d", &idx); err == nil && idx >= 1 && idx <= len(opts) {
				return opts[idx-1]
			}
			fmt.Printf("  无效输入，请输入 1-%d\n", len(opts))
		}
	}

	fmt.Println("\n--- 时段配置 ---")
	return SlotConfig{
		Weekday:  choose("工作日 (周一-周五)"),
		Saturday: choose("周六"),
		Sunday:   choose("周日"),
	}
}
