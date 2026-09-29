package main

import (
	"context"
)

type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Ping returns a simple status message, demonstrating Go ↔ frontend communication.
func (a *App) Ping() string {
	return "pong"
}

// GetConnectionStatus returns a placeholder connection status for the Xray tunnel.
func (a *App) GetConnectionStatus() string {
	// TODO: integrate real Xray status here.
	return "disconnected"
}
