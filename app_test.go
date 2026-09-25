package main

import "testing"

func TestPing(t *testing.T) {
	app := NewApp()
	if got := app.Ping(); got != "pong" {
		t.Errorf("Ping() = %q, want %q", got, "pong")
	}
}
