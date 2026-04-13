package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const defaultWebPort = 8081

func cmdWeb() {
	printBanner()

	if checkStaleProxy() {
		fmt.Println("已清除上次异常退出的系统代理设置")
	}

	setNotifier(BuildNotifierFromConfig())

	mux := http.NewServeMux()

	// Static
	mux.HandleFunc("/", handleIndex)

	// Status & info
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/stores", handleStores)

	// Calendar & reservations
	mux.HandleFunc("/api/calendar", handleCalendar)
	mux.HandleFunc("/api/reservations", handleReservations)

	// Preferences
	mux.HandleFunc("/api/preferences", handlePreferences)

	// Notifications config
	mux.HandleFunc("/api/config", handleNotifyConfig)

	// Engine control
	mux.HandleFunc("/api/engine/state", handleEngineState)
	mux.HandleFunc("/api/engine/capture", handleEngineCapture)
	mux.HandleFunc("/api/engine/booking", handleEngineBooking)
	mux.HandleFunc("/api/engine/stop", handleEngineStop)
	mux.HandleFunc("/api/engine/logs", handleEngineLogs)

	// Sniper
	mux.HandleFunc("/api/sniper/start", handleSniperStart)

	// SSE
	mux.HandleFunc("/api/events", handleEvents)

	port := findAvailablePort(defaultWebPort)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Prepare settings for calendar/stores APIs if config exists
	tokens, ok := tryLoadConfig()
	if ok {
		prefs := LoadPreferences()
		settings := tokens.toSettingsWithPrefs(prefs)
		setWebSettings(settings)
	}

	go func() {
		<-ctx.Done()
		engine.Stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	url := fmt.Sprintf("http://%s", addr)
	fmt.Printf("Web UI 启动于 %s\n", url)
	fmt.Println("在浏览器中操作，按 Ctrl+C 退出")

	_ = OpenBrowser(url)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func findAvailablePort(preferred int) int {
	for port := preferred; port < preferred+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	return preferred
}

var (
	webSettingsMu sync.RWMutex
	webSettings   Settings
	webClient     *Client
)

func setWebSettings(s Settings) {
	webSettingsMu.Lock()
	webSettings = s
	webClient = NewClient(s)
	webSettingsMu.Unlock()
}

func getWebSettings() Settings {
	webSettingsMu.RLock()
	defer webSettingsMu.RUnlock()
	return webSettings
}

func getWebClient() *Client {
	webSettingsMu.RLock()
	defer webSettingsMu.RUnlock()
	return webClient
}

func refreshWebClient() {
	tokens, err := loadLocalConfig()
	if err != nil {
		return
	}
	prefs := LoadPreferences()
	settings := tokens.toSettingsWithPrefs(prefs)
	setWebSettings(settings)
}
