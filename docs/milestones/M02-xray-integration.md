# M02 — Xray Integration

## Objective

Integrate Xray Core with the Go backend through a dedicated service that manages the Xray process lifecycle.

This milestone focuses on backend process management. Connection UI, profile management, and release packaging are handled by later milestones.

## Deliverables

- Xray process management abstraction.
- Start and stop lifecycle.
- Process state tracking.
- Single-instance enforcement.
- Graceful application shutdown.
- Unit tests for process lifecycle behavior.

## Tasks

### 1. Create the Xray Module

Maintain a dedicated package responsible for managing the Xray process.

Suggested structure:

```text
internal/
└── xray/
    ├── errors.go
    ├── process.go
    ├── service.go
    └── state.go
```

### 2. Process Management

Implement a service capable of:

- Starting Xray with a configuration file.
- Stopping the running process.
- Preventing multiple instances.
- Tracking process termination, including unexpected exits.
- Exposing the current process state.

### 3. Lifecycle Management

The application must terminate its Xray child process when the desktop application shuts down.

Process termination must be handled safely, without leaving an orphaned Xray process.

### 4. Error Handling

Provide typed errors for expected failure conditions, including:

- Already running.
- Not running.
- Invalid executable.
- Process start failure.
- Invalid configuration file.

### 5. Testing

Add unit tests covering:

- Start and stop behavior.
- Repeated start and stop calls.
- Missing executable and configuration files.
- Unexpected process termination.
- Single-instance enforcement.
- Graceful and repeated shutdown.

## Definition of Done

- Xray can be started programmatically.
- Xray can be stopped programmatically.
- Only one instance can run at a time.
- The current process state remains accurate.
- Application shutdown terminates the Xray process cleanly.
- Unit tests pass.
- Race detection and static analysis pass.

## Implementation

Completed in `internal/xray`:

- `state.go` — Defines `StateStopped` and `StateRunning`, with a `String()` method.
- `errors.go` — Defines typed errors for process and configuration failures.
- `process.go` — Starts Xray with `xray run -c <config>`, tracks the child process, and handles process termination.
- `service.go` — Exposes the process lifecycle through `Start`, `Stop`, `State`, and `Shutdown`.

The application holds an `*xray.Service` and calls `Shutdown()` during application shutdown. `App.GetConnectionStatus()` returns the actual engine state.

Tests use a mock executable to verify lifecycle behavior.

Verification completed:

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- Release binary build.

## Runtime and Packaging Notes

- Xray is a standalone executable, not embedded into the Go executable.
- `DefaultBinaryPath()` currently resolves the executable next to the application executable.
- The final release packaging process must provide the correct Xray executable for each supported OS and CPU architecture.
- Platform-specific runtime paths and executable permissions must be verified during M05.
- The Xray binary is not committed to the source repository.

## Out of Scope

- Connection UI.
- Profile import and management.
- Release packaging and distribution.

These are handled by M03, M04, M05, and M06.