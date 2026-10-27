# M04 — Import & Profiles

## Objective

Implement configuration import and persistent profile management so users can manage their Xray connections directly in Packet Drift.

## Deliverables

- Xray JSON configuration import.
- Persistent profile storage.
- Profile selection.
- Import proxy servers from subscription URLs.
- Group multiple proxy servers in a profile.
- Active profile management.
- Configuration validation.
- Integration with the connection lifecycle.

## Tasks

### 1. Configuration Import

- Import proxy servers from subscription URLs.
- Group multiple proxy servers in a profile.
Allow users to import supported Xray JSON configuration files through a file picker.

The application must:

- Read the selected file.
- Validate its structure.
- Report invalid or unsupported configurations clearly.
- Avoid silently modifying imported configurations.
- Avoid starting Xray before validation succeeds.

### 2. Profile Model

Define a profile model containing the minimum required metadata, such as:

- Stable profile identifier.
- Display name.
- Configuration data or a managed configuration file reference.
- Creation and update metadata, where useful.

Keep the model extensible without introducing unnecessary complexity.

### 3. Persistent Storage

Store imported profiles in an application-specific user data directory.

Keep user data separate from application installation files and bundled Xray resources.

Protect configuration contents from accidental exposure, and avoid logging credentials or other sensitive configuration values.

### 4. Profile Management

Implement:

- Listing available profiles.
- Selecting a profile.
- Renaming a profile.
- Removing a profile.
- Remembering the selected profile across application restarts.

If a profile is currently connected, prevent changes that could corrupt the active runtime configuration.

### 5. Connection Integration

Use the selected profile's configuration when starting Xray.

Validate the selected configuration and report errors before attempting to establish the connection.

Ensure that switching profiles while connected follows a predictable and safe lifecycle.

## Definition of Done

- Users can import valid Xray JSON configurations.
- Invalid configurations produce understandable errors.
- Profiles persist across application restarts.
- Users can select, rename, and remove profiles.
- Users can select a server within a profile.
- The selected profile is used for new connections.
- Subscription URLs can be fetched into profiles containing multiple supported proxy servers.
- Profile changes cannot silently corrupt a running connection.
- Sensitive configuration values are not unnecessarily logged.
- Relevant unit and integration tests pass.

## Out of Scope

- Automatic subscription refresh.
- Cloud synchronization.
- Advanced configuration editing.

These features may be considered after the MVP.