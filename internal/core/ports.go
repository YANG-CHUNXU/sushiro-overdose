package core

import (
	"fmt"
	"net"
	"strconv"
)

func ListenOnAvailableLocalPort(preferred, attempts int) (net.Listener, int, error) {
	return ListenOnAvailableHostPort("127.0.0.1", preferred, attempts)
}

func ListenOnAvailableHostPort(host string, preferred, attempts int) (net.Listener, int, error) {
	var lastErr error
	for port := preferred; port < preferred+attempts; port++ {
		ln, err := listenHostPort(host, port)
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

func FirstAvailableLocalPort(preferred, attempts int) (int, bool) {
	ln, port, err := ListenOnAvailableLocalPort(preferred, attempts)
	if err != nil {
		return 0, false
	}
	_ = ln.Close()
	return port, true
}

func listenLocalPort(port int) (net.Listener, error) {
	return listenHostPort("127.0.0.1", port)
}

func listenHostPort(host string, port int) (net.Listener, error) {
	if host == "" {
		host = "127.0.0.1"
	}
	return net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
}
