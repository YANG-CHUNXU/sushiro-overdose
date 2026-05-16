package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBufferedConnReadsDataAlreadyBufferedAfterConnect(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		_, err := client.Write([]byte("CONNECT crm-cn-prd.sushiro.com.cn:443 HTTP/1.1\r\nHost: crm-cn-prd.sushiro.com.cn:443\r\n\r\nHELLO"))
		errCh <- err
	}()

	reader := bufio.NewReader(server)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read CONNECT header: %v", err)
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	got := make([]byte, 5)
	if _, err := io.ReadFull(&bufferedConn{Conn: server, reader: reader}, got); err != nil {
		t.Fatalf("read buffered payload: %v", err)
	}
	if string(got) != "HELLO" {
		t.Fatalf("buffered payload = %q, want HELLO", string(got))
	}
	if err := <-errCh; err != nil {
		t.Fatalf("client write: %v", err)
	}
}

func TestPlainHTTPProxyForwardsAbsoluteFormRequest(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" || r.URL.RawQuery != "x=1" {
			t.Fatalf("unexpected upstream URL: %s", r.URL.String())
		}
		if got := r.Header.Get("X-Test"); got != "yes" {
			t.Fatalf("upstream X-Test = %q, want yes", got)
		}
		w.Header().Set("X-Upstream", "ok")
		_, _ = w.Write([]byte("pong"))
	}))
	defer upstream.Close()

	client, server := net.Pipe()
	defer client.Close()

	ps := &proxyServer{transport: http.DefaultTransport.(*http.Transport).Clone()}
	done := make(chan struct{})
	go func() {
		ps.handleConn(server)
		close(done)
	}()

	if _, err := fmt.Fprintf(client, "GET %s/ping?x=1 HTTP/1.1\r\nHost: %s\r\nX-Test: yes\r\nConnection: close\r\n\r\n", upstream.URL, strings.TrimPrefix(upstream.URL, "http://")); err != nil {
		t.Fatalf("write request: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("X-Upstream") != "ok" {
		t.Fatalf("missing upstream header")
	}
	if string(body) != "pong" {
		t.Fatalf("body = %q, want pong", string(body))
	}
	<-done
}
