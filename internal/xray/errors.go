package xray

import "errors"

// Typed errors returned by the Xray service.
var (
	// ErrAlreadyRunning is returned when Start is called while Xray is running.
	ErrAlreadyRunning = errors.New("xray: already running")
	// ErrNotRunning is returned when Stop is called while Xray is not running.
	ErrNotRunning = errors.New("xray: not running")
	// ErrInvalidExecutable is returned when the Xray binary cannot be found.
	ErrInvalidExecutable = errors.New("xray: invalid executable")
	// ErrStartFailed is returned when the Xray process fails to start.
	ErrStartFailed = errors.New("xray: process start failed")
	// ErrInvalidConfig is returned when a config file or pasted config is invalid.
	ErrInvalidConfig = errors.New("xray: invalid config path")
	// ErrStatsUnavailable is returned when traffic stats are not configured.
	ErrStatsUnavailable = errors.New("xray: traffic stats unavailable")
)
