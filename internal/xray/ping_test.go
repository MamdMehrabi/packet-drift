package xray

import (
	"fmt"
	"net"
	"testing"
)

func TestCheckServerLatency(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	uri := fmt.Sprintf("vless://11111111-1111-4111-8111-111111111111@127.0.0.1:%d", port)
	latency, err := CheckServerLatency(uri)
	if err != nil {
		t.Fatalf("CheckServerLatency(): %v", err)
	}
	if latency < 0 {
		t.Fatalf("latency = %d, want non-negative duration", latency)
	}
}

func TestCheckServerLatencyRejectsInvalidURI(t *testing.T) {
	if _, err := CheckServerLatency("vless://invalid"); err == nil {
		t.Fatal("CheckServerLatency() accepted an invalid proxy URI")
	}
}

func TestCheckServerLatencyReportsUnreachableServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	uri := fmt.Sprintf("vless://11111111-1111-4111-8111-111111111111@127.0.0.1:%d", port)
	if _, err := CheckServerLatency(uri); err == nil {
		t.Fatal("CheckServerLatency() accepted an unreachable server")
	}
}
