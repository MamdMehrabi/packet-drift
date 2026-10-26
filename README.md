# packet-drift

Packet Drift is a Wails desktop client for running an Xray-core profile.

## Current Runtime

The repository currently bundles the official Xray-core v26.3.27 release for Linux amd64 in `internal/xray/assets/linux-amd64/`, including its geo data and MPL-2.0 license. The source and release are published at [XTLS/Xray-core](https://github.com/XTLS/Xray-core/releases/tag/v26.3.27). Other operating systems and architectures need platform-specific binary packaging, which is not included yet.

Start the desktop app with `wails dev`, paste a `vless://`, `vmess://`, `trojan://`, or `ss://` proxy URI from the system clipboard, and select Connect. Packet Drift converts the URI into an Xray configuration with a local SOCKS inbound on `127.0.0.1:10808`, then adds a loopback-only StatsService and outbound traffic counters to a private temporary copy. The pasted URI is not modified, and the temporary config is removed when disconnected or when the app exits.

The dashboard samples Xray outbound counters once per second and shows upload/download rates and totals. The profile still needs an inbound and routing setup appropriate for the clients that should use the proxy; starting the core does not change system-wide proxy settings.