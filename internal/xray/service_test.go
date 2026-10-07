package xray

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// writeFakeBinary creates a shell script that mimics a long-running
// Xray process, so the service logic can be tested without the real binary.
func writeFakeBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-xray")
	script := "#!/bin/sh\nsleep 10\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return bin
}

func writeConfig(t *testing.T) string {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfg, []byte(`{"log":{"level":"silent"}}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return cfg
}

func TestStartStop(t *testing.T) {
	s := NewService()
	bin := writeFakeBinary(t)
	cfg := writeConfig(t)

	if s.State() != StateStopped {
		t.Fatalf("initial state = %s, want stopped", s.State())
	}

	if err := s.Start(bin, cfg); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	if s.State() != StateRunning {
		t.Fatalf("state after Start = %s, want running", s.State())
	}

	// Give the fake process a moment, then stop it.
	time.Sleep(50 * time.Millisecond)
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop(): %v", err)
	}
	if s.State() != StateStopped {
		t.Fatalf("state after Stop = %s, want stopped", s.State())
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	s := NewService()
	bin := writeFakeBinary(t)
	cfg := writeConfig(t)

	if err := s.Start(bin, cfg); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	defer s.Stop()

	if err := s.Start(bin, cfg); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Start() = %v, want ErrAlreadyRunning", err)
	}
}

func TestStopNotRunning(t *testing.T) {
	s := NewService()
	if err := s.Stop(); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Stop() = %v, want ErrNotRunning", err)
	}
}

func TestStartInvalidExecutable(t *testing.T) {
	s := NewService()
	cfg := writeConfig(t)
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	err := s.Start(missing, cfg)
	if !errors.Is(err, ErrInvalidExecutable) {
		t.Fatalf("Start() = %v, want ErrInvalidExecutable", err)
	}
	if s.State() != StateStopped {
		t.Fatalf("state = %s, want stopped after failed Start", s.State())
	}
}

func TestStartInvalidConfig(t *testing.T) {
	s := NewService()
	bin := writeFakeBinary(t)
	missing := filepath.Join(t.TempDir(), "no-config.json")

	err := s.Start(bin, missing)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Start() = %v, want ErrInvalidConfig", err)
	}
}

func TestShutdown(t *testing.T) {
	s := NewService()
	bin := writeFakeBinary(t)
	cfg := writeConfig(t)

	if err := s.Start(bin, cfg); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	s.Shutdown() // must not return an error and must clear state
	if s.State() != StateStopped {
		t.Fatalf("state after Shutdown = %s, want stopped", s.State())
	}

	// Shutdown again: still safe.
	s.Shutdown()
	if s.State() != StateStopped {
		t.Fatalf("state after double Shutdown = %s, want stopped", s.State())
	}
}

func TestProcessSingleInstance(t *testing.T) {
	p := NewProcess()
	bin := writeFakeBinary(t)
	cfg := writeConfig(t)

	if err := p.Start(bin, cfg); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	if !p.IsRunning() {
		t.Fatal("IsRunning() = false after Start, want true")
	}
	if err := p.Start(bin, cfg); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Start() = %v, want ErrAlreadyRunning", err)
	}
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop(): %v", err)
	}
	if p.IsRunning() {
		t.Fatal("IsRunning() = true after Stop, want false")
	}
}

func TestProcessExitsOnItsOwn(t *testing.T) {
	// A binary that exits immediately must not keep reporting as running.
	dir := t.TempDir()
	bin := filepath.Join(dir, "short-lived")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	cfg := writeConfig(t)

	p := NewProcess()
	if err := p.Start(bin, cfg); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if p.IsRunning() {
		t.Fatal("IsRunning() = true after process exited on its own, want false")
	}
}

func TestBundledXrayStatsAPI(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("bundled Xray release is linux/amd64 only")
	}
	binaryPath, err := DefaultBinaryPath()
	if err != nil {
		t.Fatalf("DefaultBinaryPath(): %v", err)
	}
	body := make([]byte, 64*1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()
	serverHost, serverPort, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatalf("parse local HTTP server address: %v", err)
	}
	serverPortNumber, err := strconv.Atoi(serverPort)
	if err != nil {
		t.Fatalf("parse local HTTP server port: %v", err)
	}
	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve SOCKS port: %v", err)
	}
	_, proxyPort, err := net.SplitHostPort(proxyListener.Addr().String())
	_ = proxyListener.Close()
	if err != nil {
		t.Fatalf("read SOCKS port: %v", err)
	}
	service := NewService()
	config := fmt.Sprintf(`{"log":{"loglevel":"warning"},"inbounds":[{"listen":"127.0.0.1","port":%s,"protocol":"socks","settings":{"auth":"noauth","udp":false},"tag":"socks"}],"outbounds":[{"protocol":"freedom","tag":"direct"}]}`, proxyPort)
	if err := service.StartWithConfig(binaryPath, config); err != nil {
		t.Fatalf("StartWithConfig(): %v", err)
	}
	t.Cleanup(service.Shutdown)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if service.State() != StateRunning {
			t.Fatal("bundled Xray exited while loading instrumented config")
		}
		if _, err := service.Traffic(); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if time.Now().After(deadline) {
		t.Fatal("Xray StatsService did not become available")
	}

	proxyAddress := net.JoinHostPort("127.0.0.1", proxyPort)
	connection, err := net.DialTimeout("tcp", proxyAddress, 2*time.Second)
	if err != nil {
		t.Fatalf("connect to Xray SOCKS inbound: %v", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := connection.Write([]byte{5, 1, 0}); err != nil {
		t.Fatalf("write SOCKS greeting: %v", err)
	}
	methodReply := make([]byte, 2)
	if _, err := io.ReadFull(connection, methodReply); err != nil || methodReply[1] != 0 {
		t.Fatalf("SOCKS auth negotiation = %v, error = %v", methodReply, err)
	}
	serverIP := net.ParseIP(serverHost).To4()
	if serverIP == nil {
		t.Fatalf("local test server host is not IPv4: %q", serverHost)
	}
	if _, err := connection.Write([]byte{5, 1, 0, 1, serverIP[0], serverIP[1], serverIP[2], serverIP[3], byte(serverPortNumber >> 8), byte(serverPortNumber)}); err != nil {
		t.Fatalf("write SOCKS connect request: %v", err)
	}
	connectReply := make([]byte, 4)
	if _, err := io.ReadFull(connection, connectReply); err != nil || connectReply[1] != 0 {
		t.Fatalf("SOCKS connect reply = %v, error = %v", connectReply, err)
	}
	addressLength := 6
	switch connectReply[3] {
	case 1:
		addressLength = 6
	case 3:
		length := []byte{0}
		if _, err := io.ReadFull(connection, length); err != nil {
			t.Fatalf("read SOCKS domain length: %v", err)
		}
		addressLength = int(length[0]) + 2
	case 4:
		addressLength = 18
	default:
		t.Fatalf("unknown SOCKS address type: %d", connectReply[3])
	}
	if _, err := io.CopyN(io.Discard, connection, int64(addressLength)); err != nil {
		t.Fatalf("read SOCKS connect address: %v", err)
	}
	if _, err := fmt.Fprintf(connection, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", serverHost); err != nil {
		t.Fatalf("write HTTP request through SOCKS: %v", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatalf("read HTTP response through SOCKS: %v", err)
	}
	responseBody, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || len(responseBody) != len(body) {
		t.Fatalf("HTTP body length = %d, error = %v, want %d", len(responseBody), err, len(body))
	}

	deadline = time.Now().Add(5 * time.Second)
	var lastStats TrafficStats
	for time.Now().Before(deadline) {
		stats, err := service.Traffic()
		lastStats = stats
		if err == nil && stats.DownloadBytes >= uint64(len(body)) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Xray outbound download counter did not include the proxied response: %#v", lastStats)
}

func TestBundledXrayRejectsInvalidProfile(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("bundled Xray release is linux/amd64 only")
	}
	binaryPath, err := DefaultBinaryPath()
	if err != nil {
		t.Fatalf("DefaultBinaryPath(): %v", err)
	}
	service := NewService()
	err = service.StartWithConfig(binaryPath, `{"outbounds":[{"protocol":"not-a-protocol"}]}`)
	if err == nil || !strings.Contains(err.Error(), "validate config") {
		t.Fatalf("StartWithConfig() error = %v, want Xray config validation error", err)
	}
	if service.State() != StateStopped {
		t.Fatalf("state after invalid config = %s, want stopped", service.State())
	}
}
