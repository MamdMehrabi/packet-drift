package main

import (
	"context"
	"fmt"

	"github.com/MamdMehrabi/packet-drift/internal/xray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx  context.Context
	xray *xray.Service
}

func NewApp() *App {
	return &App{xray: xray.NewService()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.xray.SetLogHandler(func(line string) {
		runtime.EventsEmit(ctx, "xray:log", line)
	})
}

func (a *App) shutdown(ctx context.Context) {
	// Terminate the Xray process cleanly when the desktop app closes.
	a.xray.Shutdown()
}

// Ping returns a simple status message, demonstrating Go ↔ frontend communication.
func (a *App) Ping() string {
	return "pong"
}

// GetConnectionStatus returns the current Xray engine state as a UI-safe string.
func (a *App) GetConnectionStatus() string {
	return a.xray.State().String()
}

// GetXrayBinaryPath returns the conventional Xray binary location.
func (a *App) GetXrayBinaryPath() (string, error) {
	return xray.DefaultBinaryPath()
}

// GetLANAddresses returns private IPv4 addresses available for LAN sharing.
func (a *App) GetLANAddresses() []string {
	return xray.LANAddresses()
}

// SelectConfigFile opens a native file picker for an Xray JSON config.
func (a *App) SelectConfigFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Xray configuration",
		Filters: []runtime.FileFilter{{
			DisplayName: "JSON configuration",
			Pattern:     "*.json",
		}},
	})
}

// SelectXrayBinary opens a native file picker for an Xray executable.
func (a *App) SelectXrayBinary() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Xray executable",
	})
}

// GetClipboardText reads the current system clipboard for config paste.
func (a *App) GetClipboardText() (string, error) {
	return runtime.ClipboardGetText(a.ctx)
}

// FetchSubscription downloads and parses a supported proxy subscription.
func (a *App) FetchSubscription(subscriptionURL string) ([]xray.SubscriptionNode, error) {
	return xray.FetchSubscription(subscriptionURL)
}

// PingServer measures the TCP connection latency for a supported proxy URI.
func (a *App) PingServer(proxyURI string) (int64, error) {
	return xray.CheckServerLatency(proxyURI)
}

// StartXray starts the engine with the selected binary and config file.
// An empty binaryPath uses the conventional location next to the app.
func (a *App) StartXray(binaryPath, configPath string) error {
	if binaryPath == "" {
		var err error
		binaryPath, err = xray.DefaultBinaryPath()
		if err != nil {
			return err
		}
	}
	return a.xray.Start(binaryPath, configPath)
}

// StopXray stops the active Xray process.
func (a *App) StopXray() error {
	return a.xray.Stop()
}

// StartXrayWithURI converts a proxy URI to an Xray config and starts the engine.
func (a *App) StartXrayWithURI(binaryPath, proxyURI, mode, proxyAddress string) error {
	if mode == "lan" && !xray.IsLANAddress(proxyAddress) {
		return fmt.Errorf("%w: selected LAN address is no longer available on this device", xray.ErrInvalidConfig)
	}
	if binaryPath == "" {
		var err error
		binaryPath, err = xray.DefaultBinaryPath()
		if err != nil {
			return err
		}
	}
	return a.xray.StartWithURI(binaryPath, proxyURI, mode, proxyAddress)
}

// GetTrafficStats returns cumulative Xray outbound traffic and current rates.
func (a *App) GetTrafficStats() (xray.TrafficStats, error) {
	return a.xray.Traffic()
}
