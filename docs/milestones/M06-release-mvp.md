# M06 — Release MVP

## Objective

Build, verify, and publish the first usable Packet Drift release for the supported desktop platforms.

## Deliverables

- Repeatable release build process.
- Platform-specific application packages.
- Release automation through CI.
- Installation and runtime smoke tests.
- Versioned release artifacts.
- Basic user-facing documentation.

## Tasks

### 1. Release Build Matrix

Build and package the application for every supported target:

- macOS ARM64.
- macOS AMD64.
- Linux AMD64.
- Windows AMD64.

Use appropriate build environments and platform-specific packaging tools.

### 2. Continuous Integration

Configure CI to:

- Run Go tests and static analysis.
- Run available frontend checks and tests.
- Build the application for supported targets.
- Acquire and verify the pinned Xray runtime.
- Produce the appropriate platform-specific packages.
- Fail the release pipeline when required checks fail.

Keep release artifacts separate from ordinary development builds.

### 3. Installation Verification

Test the actual packaged application rather than relying only on successful compilation.

Verify:

- The application installs or launches correctly.
- The bundled Xray executable is present.
- The executable can be launched by Packet Drift.
- The user can import a configuration.
- The user can connect and disconnect.
- Application shutdown terminates the Xray process.
- The application works without downloading Xray at launch.

### 4. Platform-Specific Requirements

Verify platform-specific installation and execution behavior.

For macOS, assess code signing, application packaging, and Gatekeeper compatibility.

For Windows, verify executable launching, application packaging, and security-related execution failures.

For Linux, verify executable permissions, supported package formats, and runtime library requirements.

Do not claim support for a target that has not been tested.

### 5. Release Documentation

Provide:

- Installation instructions.
- Supported platform information.
- Basic usage instructions.
- Known limitations.
- A changelog.
- Instructions for reporting bugs.

### 6. Publish Release

Publish versioned release artifacts and their checksums.

Clearly identify the supported platforms and architectures.

## Definition of Done

- CI produces the required release artifacts.
- All supported release targets pass their required smoke tests.
- Xray is bundled with every published package.
- The core import, connect, and disconnect workflow works.
- No manual Xray installation is required.
- Release notes and installation instructions are available.
- The MVP is published as a versioned release.

## Out of Scope

- Automatic application updates.
- Automatic Xray updates after release.
- Advanced diagnostics.
- Subscription management.

These can be introduced after the MVP is usable.