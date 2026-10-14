# M03 — Connection UI

## Objective

Build the initial connection interface and connect it to the existing Go backend.

The UI must reflect the actual Xray process state rather than maintaining an independent, potentially inaccurate connection state.

## Deliverables

- Main application window.
- Connect and disconnect controls.
- Connection status indicator.
- Loading and error states.
- Frontend-to-backend connection lifecycle integration.

## Tasks

### 1. Main Interface

Create a minimal interface consistent with Packet Drift's visual identity.

The interface should prioritize connection status and the primary connection action.

### 2. Connection Controls

Implement:

- Connect action.
- Disconnect action.
- Disabled controls when an operation cannot be performed.
- Protection against repeated or conflicting requests.

### 3. Connection State

Display the actual state reported by the backend.

Handle at least:

- Disconnected.
- Connecting.
- Connected.
- Disconnecting.
- Error.

The frontend may represent transitional states, but the backend remains authoritative about whether the Xray process is running.

A running process must not automatically be treated as proof that the proxy is fully usable.

### 4. Error Handling

Display understandable errors when:

- Xray fails to start.
- The configuration is invalid.
- The process exits unexpectedly.
- A connection operation fails.

Avoid exposing raw internal errors directly to the user unless useful diagnostic details are provided separately.

### 5. Backend Integration

Expose the necessary methods through the Wails bridge.

Use a temporary development configuration for testing if profile import is not yet implemented.

Replace the temporary configuration flow with profile selection in M04.

## Definition of Done

- The user can request a connection from the UI.
- The user can disconnect from the UI.
- The UI reflects backend process state accurately.
- Loading and error states are handled.
- Repeated clicks do not create multiple Xray processes.
- The application remains usable after an operation fails.
- Frontend and backend tests pass where applicable.

## Out of Scope

- Importing configurations.
- Persistent profile management.
- Release packaging.

These are handled by M04 and later milestones.