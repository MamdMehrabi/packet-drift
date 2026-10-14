package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPing(t *testing.T) {
	app := NewApp()
	if got := app.Ping(); got != "pong" {
		t.Errorf("Ping() = %q, want %q", got, "pong")
	}
}

func TestXrayLifecycleThroughApp(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "xray")
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	t.Cleanup(func() { app.shutdown(nil) })
	if err := app.StartXray(binaryPath, configPath); err != nil {
		t.Fatalf("StartXray() error = %v", err)
	}
	if got := app.GetConnectionStatus(); got != "running" {
		t.Fatalf("GetConnectionStatus() = %q, want %q", got, "running")
	}
	if err := app.StopXray(); err != nil {
		t.Fatalf("StopXray() error = %v", err)
	}
	if got := app.GetConnectionStatus(); got != "stopped" {
		t.Fatalf("GetConnectionStatus() = %q, want %q", got, "stopped")
	}
}
