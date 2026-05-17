package main

import (
	"fmt"
	"net/http"
	"strconv"
)

func handleProxyPAC(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.Atoi(r.URL.Query().Get("proxy"))
	if err != nil || port <= 0 || port > 65535 {
		http.Error(w, "invalid proxy port", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, `function FindProxyForURL(url, host) {
  host = String(host || "").toLowerCase().replace(/\.$/, "");
  if (host === "%s") {
    return "PROXY 127.0.0.1:%d";
  }
  return "DIRECT";
}
`, sushiroHost, port)
}
