# M02 – Xray Integration

**Milestone Goal**: Provide a visible connection‑status indicator for the Xray tunnel inside the Packet Drift UI.

## Tasks & Deliverables

| # | Task | Description |
|---|------|-------------|
- Backend method `GetConnectionStatus` | Add a placeholder method to `app.go` that returns a string (`"disconnected"`). Later replace with real Xray status logic.
- Extend Zustand store | Add `connectionStatus` state and `setConnectionStatus` setter to `frontend/src/store/app.ts`.
- Dashboard UI update | Import `GetConnectionStatus`, call it on mount with `useEffect`, store the result, and display `Connection status: …` below the existing Ping status.
- System dependency | Install `libwebkit2gtk-4.0-dev` (or distro‑equivalent) so `wails dev` can compile the WebView backend.
- Verify end‑to‑end | Run `wails dev`, open the dev UI (`http://localhost:34115`), confirm the status text appears and updates when the placeholder value is changed.
- Documentation update | Add this MD file to `docs/milestones/` and reference it from the roadmap.

## Acceptance Criteria
1. `GetConnectionStatus` is callable from the frontend (`../../wailsjs/go/main/App`).
2. UI shows **"Connection status: disconnected"** (or the placeholder value) on initial load.
3. Changing the returned string in Go to `"connected"` results in UI reflecting the new value after a restart.
4. No runtime errors; `wails dev` builds successfully after installing the required system package.

## Next Steps (Future M02 extensions)
- Implement real Xray client integration (start/stop, config loading).
- Add user‑configurable path for the Xray config file (default `$HOME/.config/packet-drift/config.json`).
- Write unit tests for the new Go method and React component.

---
