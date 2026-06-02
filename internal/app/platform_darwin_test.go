//go:build darwin

package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDarwinSetSystemProxyCommandsUsePACWhenWebPortAvailable(t *testing.T) {
	commands := darwinSetSystemProxyCommands([]string{"Wi-Fi"}, 8080, 52123)
	want := [][]string{
		{"networksetup", "-setautoproxyurl", "Wi-Fi", "http://127.0.0.1:52123/proxy.pac?proxy=8080"},
		{"networksetup", "-setautoproxystate", "Wi-Fi", "on"},
		{"networksetup", "-setwebproxystate", "Wi-Fi", "off"},
		{"networksetup", "-setsecurewebproxystate", "Wi-Fi", "off"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands mismatch\nwant: %#v\n got: %#v", want, commands)
	}
}

func TestDarwinSetSystemProxyCommandsDisablePACBeforeManualProxy(t *testing.T) {
	commands := darwinSetSystemProxyCommands([]string{"Wi-Fi"}, 8080, 0)
	want := [][]string{
		{"networksetup", "-setautoproxystate", "Wi-Fi", "off"},
		{"networksetup", "-setwebproxy", "Wi-Fi", "127.0.0.1", "8080"},
		{"networksetup", "-setsecurewebproxy", "Wi-Fi", "127.0.0.1", "8080"},
		{"networksetup", "-setwebproxystate", "Wi-Fi", "on"},
		{"networksetup", "-setsecurewebproxystate", "Wi-Fi", "on"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands mismatch\nwant: %#v\n got: %#v", want, commands)
	}
}

func TestDarwinClearSystemProxyCommandsDisablePACAndManualProxy(t *testing.T) {
	commands := darwinClearSystemProxyCommands([]string{"Wi-Fi"})
	want := [][]string{
		{"networksetup", "-setautoproxystate", "Wi-Fi", "off"},
		{"networksetup", "-setwebproxystate", "Wi-Fi", "off"},
		{"networksetup", "-setsecurewebproxystate", "Wi-Fi", "off"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands mismatch\nwant: %#v\n got: %#v", want, commands)
	}
}

func TestDarwinRunSystemProxyCommandsContinuesAndReturnsFailures(t *testing.T) {
	commands := [][]string{
		{"networksetup", "-setautoproxyurl", "Wi-Fi", "http://127.0.0.1:52123/proxy.pac?proxy=8080"},
		{"networksetup", "-setautoproxystate", "Wi-Fi", "on"},
		{"networksetup", "-setwebproxystate", "Wi-Fi", "off"},
	}
	var gotCommands [][]string

	err := darwinRunSystemProxyCommands(commands, func(name string, args ...string) (string, error) {
		command := append([]string{name}, args...)
		gotCommands = append(gotCommands, command)
		switch len(gotCommands) {
		case 1:
			return "PAC failed\n", errors.New("exit status 4")
		case 3:
			return "web proxy failed\n", errors.New("exit status 5")
		default:
			return "", nil
		}
	})

	if !reflect.DeepEqual(gotCommands, commands) {
		t.Fatalf("commands executed mismatch\nwant: %#v\n got: %#v", commands, gotCommands)
	}
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	msg := err.Error()
	for _, want := range []string{
		"networksetup -setautoproxyurl Wi-Fi http://127.0.0.1:52123/proxy.pac?proxy=8080",
		"PAC failed",
		"networksetup -setwebproxystate Wi-Fi off",
		"web proxy failed",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q does not contain %q", msg, want)
		}
	}
}
