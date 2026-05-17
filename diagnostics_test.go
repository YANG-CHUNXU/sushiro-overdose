package main

import "testing"

func TestSystemProxyMentionsPort(t *testing.T) {
	cases := []struct {
		name    string
		summary []string
		port    int
		want    bool
	}{
		{
			name:    "windows proxy server",
			summary: []string{"ProxyEnable=1", "ProxyServer=127.0.0.1:8083"},
			port:    8083,
			want:    true,
		},
		{
			name:    "darwin compact service output",
			summary: []string{"Wi-Fi HTTP[Enabled: Yes; Server: 127.0.0.1; Port: 8084]"},
			port:    8084,
			want:    true,
		},
		{
			name:    "windows pac auto config",
			summary: []string{"AutoConfigURL=http://127.0.0.1:8081/proxy.pac?proxy=8085"},
			port:    8085,
			want:    true,
		},
		{
			name:    "different port",
			summary: []string{"ProxyServer=127.0.0.1:7897"},
			port:    8080,
			want:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := systemProxyMentionsPort(DiagnosticSystemProxy{Available: true, Summary: tc.summary}, tc.port)
			if got != tc.want {
				t.Fatalf("systemProxyMentionsPort() = %t, want %t", got, tc.want)
			}
		})
	}
}
