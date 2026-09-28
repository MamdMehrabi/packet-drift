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