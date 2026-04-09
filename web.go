package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const webPort = 8081

func cmdWeb() {
	printBanner()

	tokens, ok := tryLoadConfig()
	if !ok {
		fmt.Println("暂无配置，部分功能不可用。请先运行 sushiro-overdose 获取认证参数。")
	}

	var settings Settings
	if ok {
		settings = tokens.toSettings()
	}
	globalNotifier = BuildNotifierFromConfig()

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/calendar", handleCalendar)
	mux.HandleFunc("/api/stores", handleStores)
	mux.HandleFunc("/api/config", handleGetConfig)
	mux.HandleFunc("/api/sniper/start", handleSniperStart)
	mux.HandleFunc("/api/reservations", handleReservations)
	mux.HandleFunc("/api/events", handleEvents)

	// Static files (embedded)
	mux.HandleFunc("/", handleIndex)

	addr := fmt.Sprintf("127.0.0.1:%d", webPort)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Store settings in context for handlers
	webSettings = settings

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	url := fmt.Sprintf("http://%s", addr)
	fmt.Printf("Web UI 启动于 %s\n", url)
	fmt.Println("按 Ctrl+C 退出")

	_ = OpenBrowser(url)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var webSettings Settings
