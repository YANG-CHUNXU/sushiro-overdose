package app

import (
	"fmt"
	"net"
)

func listenOnAvailableLocalPort(preferred, attempts int) (net.Listener, int, error) {
	var lastErr error
	for port := preferred; port < preferred+attempts; port++ {
		ln, err := listenLocalPort(port)
		if err == nil {
			return ln, port, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no ports checked")
	}
	return nil, 0, lastErr
}

func firstAvailableLocalPort(preferred, attempts int) (int, bool) {
	ln, port, err := listenOnAvailableLocalPort(preferred, attempts)
	if err != nil {
		return 0, false
	}
	_ = ln.Close()
	return port, true
}

func listenLocalPort(port int) (net.Listener, error) {
	return net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
}
