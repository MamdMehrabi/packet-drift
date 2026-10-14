# M05 — Runtime Packaging

## Objective

Bundle the correct standalone Xray Core executable with each platform-specific Packet Drift application package.

Users must not need to download, install, or configure Xray separately.

This milestone covers runtime acquisition and packaging, not the complete release pipeline.

## Deliverables

- Automated Xray artifact acquisition during the build.
- Explicit Xray version pinning.
- Platform and architecture mapping.
- Artifact integrity verification.
- Platform-aware executable path resolution.
- Bundled runtime integration.
- Automated packaging tests.

## Tasks

### 1. Define Supported Targets

Initially support:

- macOS ARM64.
- macOS AMD64.
- Linux AMD64.
- Windows AMD64.

Additional architectures can be introduced after the initial targets are verified.

### 2. Pin the Xray Version

Define the Xray Core version used for each application release.

Do not silently download the latest available release during builds.

Version changes must be explicit and reproducible.

### 3. Acquire Xray Artifacts

Create a build-time process that:

- Retrieves the correct official Xray release artifact.
- Maps the target OS and architecture to the appropriate artifact.
- Extracts the executable from the downloaded archive.
- Verifies the artifact against trusted checksum or signature information when available.
- Fails the build if verification fails.
- Never executes an unverified downloaded binary.

Use official Xray release assets rather than committing executable binaries to the source repository.

### 4. Bundle the Executable

Integrate the executable into each platform's application package using Wails-supported asset or packaging mechanisms.

The final location must account for platform-specific packaging rules.

The Xray executable must remain a separate process rather than being embedded into the Go executable.

### 5. Runtime Path Resolution

Update the runtime path resolution as needed.

The application must:

- Locate the bundled executable reliably.
- Verify that the expected executable exists.
- Handle executable permissions on Unix-like systems.
- Report missing or unusable runtime files clearly.
- Avoid relying on the current working directory.
- Avoid writing runtime files into read-only installation directories.

Keep bundled runtime resources separate from user profiles and mutable application data.

### 6. Build Integration

Make the packaging process repeatable and suitable for local development and CI.

Development builds may use a locally installed or explicitly supplied Xray binary, provided this behavior is documented and cannot accidentally leak into release packaging.

Release builds must fail if the required bundled runtime is missing.

### 7. Testing

Test:

- Artifact selection for every supported target.
- Artifact integrity verification.
- Runtime path resolution.
- Executable permissions.
- Missing runtime handling.
- Successful startup using the bundled executable.
- Packaging without a network connection at application launch time.

## Definition of Done

- Every supported release package contains the correct Xray executable.
- No Xray executable is committed to the source repository.
- The application locates and launches its bundled Xray executable.
- The installed application does not need to download Xray on first launch.
- Runtime integrity and version information can be verified.
- Packaging failures are reported clearly.
- All supported target combinations pass packaging and smoke tests.

## Out of Scope

- Full installer and release automation.
- Automatic application updates.
- Automatic Xray version updates after installation.

These are handled by M06 or considered for later releases.