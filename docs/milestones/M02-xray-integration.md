# M02 — Xray Integration

## Objective

Integrate the Xray core into the backend and provide a clean Go service for starting and stopping the VPN engine.

This milestone focuses only on backend infrastructure. No connection UI or profile management is included.

## Deliverables

- Embedded Xray executable management
- Xray service abstraction
- Start / Stop lifecycle
- Process state tracking
- Graceful shutdown

## Tasks

### 1. Create Xray module

Create a dedicated package responsible for managing the Xray process.

Suggested structure:

```text
backend/
internal/
  xray/
    service.go
    process.go
    state.go
```

### 2. Process management

Implement a service capable of:

- starting Xray from a config file
- stopping the running process
- preventing multiple instances
- exposing the current state

### 3. Lifecycle

The application must terminate Xray automatically when the desktop app closes.

### 4. Error handling

Return typed Go errors for:

- already running
- not running
- invalid executable
- process start failure

### 5. Tests

Add unit tests for the service logic where process execution can be mocked.

## Definition of Done

- Xray can be started programmatically.
- Xray can be stopped programmatically.
- Only one instance may run at a time.
- Current state is queryable.
- Closing the app terminates the Xray process cleanly.

## Implementation

Completed in `internal/xray`:

- `state.go` — `State` enum (`StateStopped`, `StateRunning`) with `String()` for UI display.
- `errors.go` — typed errors: `ErrAlreadyRunning`, `ErrNotRunning`, `ErrInvalidExecutable`, `ErrStartFailed`, `ErrInvalidConfig`.
- `process.go` — `Process` manages a single OS process:
  - `Start(binaryPath, configPath)` validates binary + config, launches `xray run -c <config>`, and reaps the child via a wait goroutine (state stays accurate if the process exits on its own).
  - `Stop()` kills the process and blocks until it has fully exited.
  - `IsRunning()` exposes process liveness.
  - `DefaultBinaryPath()` resolves the embedded binary next to the executable for single-file builds.
- `service.go` — `Service` wraps `Process` with `atomic` state tracking, `Start`/`Stop`/`State`/`Shutdown`. `Shutdown` is error-free and safe to call on app exit.

Wired into the app:

- `App` in `app.go` holds an `*xray.Service`; `OnShutdown` (`shutdown(ctx)`) calls `xray.Shutdown()` so closing the desktop app terminates Xray cleanly.
- `App.GetConnectionStatus()` now returns the real engine state instead of a placeholder.

Tests (`internal/xray/service_test.go`) use a shell script as a mock Xray binary:

- start/stop lifecycle, double-start rejection (`ErrAlreadyRunning`), stop-when-stopped (`ErrNotRunning`), missing binary (`ErrInvalidExecutable`), missing config (`ErrInvalidConfig`), idempotent `Shutdown`, single-instance guarantee, and self-terminating processes.

Verified: `go test ./...` and `go test -race ./...` pass; `go vet` clean; release binary builds.

## Notes

- The Xray binary is not shipped in the repo yet; it is resolved via `DefaultBinaryPath()` (next to the executable) and should be embedded during M05 packaging.
- Frontend/UI changes belong to M03 (Connect UI).