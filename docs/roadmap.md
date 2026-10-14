# Packet Drift Roadmap

Packet Drift is a minimal cross-platform desktop Xray client built with Go, Wails, React, and TypeScript.

## Goal

Build a usable desktop client that lets users import Xray configurations, manage profiles, and connect or disconnect with one click.

Xray Core is distributed as a standalone executable bundled with each platform's application package. Users should not need to install or download Xray separately.

## Milestones

| ID | Milestone | Status |
|---|---|---|
| M01 | Foundation | Completed |
| M02 | Xray Integration | Not Started |
| M03 | Connection UI | Not Started |
| M04 | Import & Profiles | Not Started |
| M05 | Runtime Packaging | Not Started |
| M06 | Release MVP | Not Started |
| M07 | Reliability & Polish | Not Started |

## Scope

### MVP

- Cross-platform desktop application for macOS, Linux, and Windows.
- Import Xray JSON configurations.
- Manage imported connection profiles.
- Connect and disconnect with one click.
- Display connection status and useful error messages.
- Bundle the appropriate Xray Core executable with each release package.
- Build and verify platform-specific release artifacts.

### Post-MVP

Additional features such as subscription management, automatic configuration updates, advanced connection diagnostics, and in-app Xray upgrades may be considered after the MVP is stable.

## Engineering Principles

- Keep Xray Core as a separate OS process.
- Keep process management independent of the frontend.
- Bundle Xray during the release build; do not commit platform binaries to the source repository.
- Pin the Xray version used for each release and verify downloaded artifacts.
- Keep application resources separate from user data and imported configurations.
- Keep platform-specific behavior behind small, testable abstractions.
- Test core logic independently of the graphical interface.

## Definition of MVP

The MVP is complete when a user can install Packet Drift on a supported platform, import a valid Xray configuration, select a profile, connect, disconnect, and see the correct connection status without manually installing Xray.

The release must include the appropriate Xray executable and pass the supported platform's build and smoke tests.