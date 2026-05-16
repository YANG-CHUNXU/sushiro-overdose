package main

import (
	"bufio"
	"io"
	"net"
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
