import { useState, useEffect, useRef } from "react";
import {
  GetClipboardText,
  GetConnectionStatus,
  GetTrafficStats,
  GetXrayBinaryPath,
  StartXrayWithURI,
  StopXray,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { useAppStore } from "../store/app";

interface TrafficStats {
  uploadBytes: number;
  downloadBytes: number;
  uploadBytesPerSecond: number;
  downloadBytesPerSecond: number;
}

type ConnectionMode = "proxy" | "tunnel";

interface ConnectionProfile {
  id: string;
  name: string;
  tag: string;
  uri: string;
}

const profilesStorageKey = "packet-drift-profiles";
const selectedProfileStorageKey = "packet-drift-selected-profile";

function storedSetting(key: string, fallback: string) {
  try {
    return window.localStorage.getItem(key) || fallback;
  } catch {
    return fallback;
  }
}

function storedProfiles(): ConnectionProfile[] {
  try {
    const parsed = JSON.parse(window.localStorage.getItem(profilesStorageKey) || "[]");
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((profile): profile is ConnectionProfile =>
      typeof profile?.id === "string" &&
      typeof profile?.name === "string" &&
      typeof profile?.tag === "string" &&
      typeof profile?.uri === "string",
    );
  } catch {
    return [];
  }
}

function profileTagFromURI(uri: string) {
  const value = uri.trim();
  if (value.toLowerCase().startsWith("vmess://")) {
    try {
      const payload = value.slice("vmess://".length).split("#", 1)[0];
      const decoded = atob(payload.replace(/-/g, "+").replace(/_/g, "/"));
      const profile = JSON.parse(new TextDecoder().decode(Uint8Array.from(decoded, (char) => char.charCodeAt(0))));
      if (typeof profile.ps === "string" && profile.ps.trim()) return profile.ps.trim();
    } catch {
      return "proxy";
    }
  }
  try {
    const parsed = new URL(value);
    const fragment = decodeURIComponent(parsed.hash.slice(1)).trim();
    return fragment || "proxy";
  } catch {
    return "proxy";
  }
}

const emptyTraffic: TrafficStats = {
  uploadBytes: 0,
  downloadBytes: 0,
  uploadBytesPerSecond: 0,
  downloadBytesPerSecond: 0,
};

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KiB", "MiB", "GiB", "TiB"];
  let amount = bytes / 1024;
  let unit = 0;
  while (amount >= 1024 && unit < units.length - 1) {
    amount /= 1024;
    unit += 1;
  }
  return `${amount.toFixed(amount >= 100 ? 0 : 1)} ${units[unit]}`;
}

function formatRate(bytesPerSecond: number) {
  return `${formatBytes(bytesPerSecond)}/s`;
}

export function Dashboard() {
  const { connectionStatus, setConnectionStatus } = useAppStore();
  const [profiles, setProfiles] = useState<ConnectionProfile[]>(storedProfiles);
  const [selectedProfileId, setSelectedProfileId] = useState(() => storedSetting(selectedProfileStorageKey, ""));
  const [profilesOpen, setProfilesOpen] = useState(false);
  const [editingProfileId, setEditingProfileId] = useState<string | null>(null);
  const [editorName, setEditorName] = useState("");
  const [editorURI, setEditorURI] = useState("");
  const [connectionMode, setConnectionMode] = useState<ConnectionMode>(() =>
    storedSetting("packet-drift-mode", "proxy") === "tunnel" ? "tunnel" : "proxy",
  );
  const [proxyAddress, setProxyAddress] = useState(() =>
    storedSetting("packet-drift-address", "127.0.0.1:10808"),
  );
  const [binaryPath, setBinaryPath] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [traffic, setTraffic] = useState<TrafficStats>(emptyTraffic);
  const [history, setHistory] = useState<TrafficStats[]>([]);
  const [logs, setLogs] = useState<string[]>([]);
  const [statsUnavailable, setStatsUnavailable] = useState(false);
  const refreshInFlight = useRef(false);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const selectedProfile = profiles.find((profile) => profile.id === selectedProfileId) ?? null;
  const proxyURI = selectedProfile?.uri ?? "";

  async function refreshDashboard() {
    if (refreshInFlight.current) return;
    refreshInFlight.current = true;
    try {
      try {
        const status = await GetConnectionStatus();
        setConnectionStatus(status);
        if (status !== "running") {
          setTraffic(emptyTraffic);
          setHistory([]);
          setStatsUnavailable(false);
          return;
        }
        try {
          const current = await GetTrafficStats();
          setTraffic(current);
          setHistory((samples) => [...samples, current].slice(-32));
          setStatsUnavailable(false);
        } catch {
          setStatsUnavailable(true);
        }
      } catch {
        setConnectionStatus("unknown");
      }
    } finally {
      refreshInFlight.current = false;
    }
  }

  useEffect(() => {
    let active = true;
    async function initialize() {
      try {
        const path = await GetXrayBinaryPath();
        if (active) setBinaryPath(path);
      } catch (err) {
        if (active) setError(String(err));
      }
      if (active) await refreshDashboard();
    }
    void initialize();
    const timer = window.setInterval(() => {
      if (active) void refreshDashboard();
    }, 1000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, []);

  useEffect(() => {
    try {
      window.localStorage.setItem(profilesStorageKey, JSON.stringify(profiles));
      window.localStorage.setItem(selectedProfileStorageKey, selectedProfileId);
      window.localStorage.setItem("packet-drift-mode", connectionMode);
      window.localStorage.setItem("packet-drift-address", proxyAddress);
    } catch {}
  }, [profiles, selectedProfileId, connectionMode, proxyAddress]);

  useEffect(() => {
    const runtimeWindow = window as Window & { runtime?: { EventsOnMultiple?: unknown } };
    if (typeof runtimeWindow.runtime?.EventsOnMultiple !== "function") return;
    return EventsOn("xray:log", (line: string) => {
      const timestamp = new Date().toLocaleTimeString();
      setLogs((entries) => [...entries, `${timestamp}  ${line}`].slice(-300));
    });
  }, []);

  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ block: "end" });
  }, [logs]);

  function openProfileManager() {
    setProfilesOpen(true);
    setEditingProfileId(null);
  }

  function startNewProfile() {
    setEditorName("");
    setEditorURI("");
    setEditingProfileId("new");
  }

  function startEditProfile(profile: ConnectionProfile) {
    setEditorName(profile.name);
    setEditorURI(profile.uri);
    setEditingProfileId(profile.id);
  }

  async function pasteEditorURI() {
    setError("");
    try {
      const clipboard = await GetClipboardText();
      if (!clipboard.trim()) {
        setError("The clipboard is empty.");
        return;
      }
      setEditorURI(clipboard.trim());
      if (!editorName.trim()) setEditorName(profileTagFromURI(clipboard.trim()));
    } catch (err) {
      setError(String(err));
    }
  }

  function saveProfile() {
    const uri = editorURI.trim();
    const tag = profileTagFromURI(uri);
    const name = editorName.trim() || tag;
    if (!uri) {
      setError("Paste a connection URI before saving the profile.");
      return;
    }
    const id = editingProfileId === "new" ? crypto.randomUUID() : editingProfileId;
    if (!id) return;
    const profile = { id, name, tag, uri };
    setProfiles((current) => {
      const index = current.findIndex((item) => item.id === id);
      if (index < 0) return [...current, profile];
      return current.map((item) => item.id === id ? profile : item);
    });
    setSelectedProfileId(id);
    setEditingProfileId(null);
    setError("");
  }

  function removeProfile(profile: ConnectionProfile) {
    if (!window.confirm(`Delete profile "${profile.name}"?`)) return;
    const remaining = profiles.filter((item) => item.id !== profile.id);
    setProfiles(remaining);
    if (selectedProfileId === profile.id) setSelectedProfileId(remaining[0]?.id ?? "");
    if (editingProfileId === profile.id) setEditingProfileId(null);
  }

  async function handleConnect() {
    if (!selectedProfile) {
      setError("Add or select a connection profile first.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await StartXrayWithURI(binaryPath, proxyURI, connectionMode, proxyAddress);
      await refreshDashboard();
    } catch (err) {
      setError(String(err));
      await refreshDashboard();
    } finally {
      setBusy(false);
    }
  }

  async function handleDisconnect() {
    setBusy(true);
    setError("");
    try {
      await StopXray();
      await refreshDashboard();
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(false);
    }
  }

  const isRunning = connectionStatus === "running";
  const maxUpload = Math.max(1, ...history.map((sample) => sample.uploadBytesPerSecond));
  const maxDownload = Math.max(1, ...history.map((sample) => sample.downloadBytesPerSecond));

  return (
    <main>
      <section className="dashboard" aria-labelledby="app-title">
        <header className="topbar">
          <div className="brand-block">
            <h1 id="app-title">Packet Drift</h1>
            <div className="selected-profile" title={selectedProfile ? `Tag: ${selectedProfile.tag}` : "No profile selected"}>
              <span>{selectedProfile?.name ?? "No profile selected"}</span>
              {selectedProfile && <small>TAG · {selectedProfile.tag}</small>}
            </div>
          </div>
          <div className={`status ${isRunning ? "status--running" : ""}`} role="status">
            <span className="status__dot" />
            {connectionStatus}
          </div>
        </header>

        <section className="connection" aria-labelledby="connection-title">
          <div>
            <p className="eyebrow">XRAY CORE</p>
            <h2 id="connection-title">{isRunning ? "Connected" : "Not connected"}</h2>
          </div>
          <p className="connection__detail">
            {isRunning
              ? `${connectionMode === "proxy" ? "Proxy" : "Tunnel"} mode is active.`
              : "Paste a proxy URI to start the engine."}
          </p>
        </section>

        <section className="connection-details" aria-label="Connection details">
          <div>
            <span className="details-label">Mode</span>
            <strong>{connectionMode === "proxy" ? "Proxy" : "Tunnel"}</strong>
          </div>
          <div>
            <span className="details-label">{connectionMode === "proxy" ? "Proxy address" : "Tunnel interface"}</span>
            <strong>{connectionMode === "proxy" ? proxyAddress || "Not set" : "packet-drift0 · 172.19.0.1/30"}</strong>
          </div>
        </section>

        <section className="traffic" aria-label="Profile outbound traffic since connection">
          <article className="traffic-panel">
            <div className="traffic-panel__heading">
              <span className="traffic-panel__label">Download from profile</span>
              <strong>{isRunning ? formatBytes(traffic.downloadBytes) : "--"}</strong>
            </div>
            <div className="traffic-chart traffic-chart--download" role="img" aria-label="Download speed history">
              {history.map((sample, index) => (
                <span key={index} style={{ height: `${Math.max(4, (sample.downloadBytesPerSecond / maxDownload) * 100)}%` }} />
              ))}
            </div>
            <span className="traffic-panel__total">{isRunning ? `${formatRate(traffic.downloadBytesPerSecond)} · current rate` : "Received since connect"}</span>
          </article>
          <article className="traffic-panel">
            <div className="traffic-panel__heading">
              <span className="traffic-panel__label">Upload to profile</span>
              <strong>{isRunning ? formatBytes(traffic.uploadBytes) : "--"}</strong>
            </div>
            <div className="traffic-chart traffic-chart--upload" role="img" aria-label="Upload speed history">
              {history.map((sample, index) => (
                <span key={index} style={{ height: `${Math.max(4, (sample.uploadBytesPerSecond / maxUpload) * 100)}%` }} />
              ))}
            </div>
            <span className="traffic-panel__total">{isRunning ? `${formatRate(traffic.uploadBytesPerSecond)} · current rate` : "Sent since connect"}</span>
          </article>
        </section>

        {statsUnavailable && isRunning && <p className="stats-note">Waiting for Xray traffic statistics...</p>}

        <section className="config-section" aria-labelledby="config-title">
          <div className="config-heading">
            <div>
              <p className="eyebrow">CONNECTION PROFILE</p>
              <h3 id="config-title">Profiles</h3>
            </div>
            <button className="button button--secondary" onClick={openProfileManager} disabled={busy || isRunning}>
              Manage
            </button>
          </div>
          <label className="profile-select-label" htmlFor="profile-select">Selected connection</label>
          <select
            id="profile-select"
            className="profile-select"
            value={selectedProfileId}
            onChange={(event) => setSelectedProfileId(event.target.value)}
            disabled={busy || isRunning || profiles.length === 0}
          >
            {profiles.length === 0 && <option value="">No saved profiles</option>}
            {profiles.map((profile) => <option key={profile.id} value={profile.id}>{profile.name} · {profile.tag}</option>)}
          </select>
        </section>

        <section className="settings-section" aria-labelledby="settings-title">
          <div className="settings-heading">
            <div>
              <h3 id="settings-title">Connection settings</h3>
            </div>
            <span className="settings-runtime">{binaryPath ? "Xray 26.3.27" : "Locating Xray..."}</span>
          </div>
          <div className="settings-grid">
            <fieldset className="mode-field" disabled={busy || isRunning}>
              <legend>Connection mode</legend>
              <div className="segmented-control">
                <button
                  type="button"
                  className={connectionMode === "proxy" ? "is-selected" : ""}
                  aria-pressed={connectionMode === "proxy"}
                  onClick={() => setConnectionMode("proxy")}
                >
                  Proxy
                </button>
                <button
                  type="button"
                  className={connectionMode === "tunnel" ? "is-selected" : ""}
                  aria-pressed={connectionMode === "tunnel"}
                  onClick={() => setConnectionMode("tunnel")}
                >
                  Tunnel
                </button>
              </div>
            </fieldset>
            {connectionMode === "proxy" ? (
              <label className="address-field">
                <span>Proxy listen address</span>
                <input
                  value={proxyAddress}
                  onChange={(event) => setProxyAddress(event.target.value)}
                  placeholder="127.0.0.1:10808"
                  disabled={busy || isRunning}
                  spellCheck={false}
                />
              </label>
            ) : (
              <p className="tunnel-note">Creates a TUN interface. System routing and network permissions are required.</p>
            )}
          </div>
        </section>

        {error && <p className="error" role="alert">{error}</p>}

        <footer className="actions">
          {isRunning ? (
            <button className="button button--disconnect" onClick={handleDisconnect} disabled={busy}>
              {busy ? "Working..." : "Disconnect"}
            </button>
          ) : (
            <button className="button button--connect" onClick={handleConnect} disabled={busy || !selectedProfile}>
              {busy ? "Connecting..." : "Connect"}
            </button>
          )}
        </footer>

        <section className="log-section" aria-labelledby="logs-title">
          <div className="log-heading">
            <div>
              <p className="eyebrow">XRAY CORE</p>
              <h3 id="logs-title">Live logs</h3>
            </div>
            <button className="log-clear" onClick={() => setLogs([])} disabled={logs.length === 0}>
              Clear
            </button>
          </div>
          <div className="log-console" role="log" aria-live="polite" aria-relevant="additions text">
            {logs.length > 0 ? logs.map((line, index) => <div className="log-line" key={`${index}-${line}`}>{line}</div>) : (
              <p className="log-empty">Xray output will appear here when the core starts.</p>
            )}
            <div ref={logsEndRef} />
          </div>
        </section>

        {profilesOpen && (
          <div className="profile-backdrop" role="presentation" onMouseDown={(event) => {
            if (event.target === event.currentTarget) setProfilesOpen(false);
          }}>
            <section className="profile-dialog" role="dialog" aria-modal="true" aria-labelledby="profile-dialog-title">
              <header className="profile-dialog__header">
                <div>
                  <p className="eyebrow">CONNECTION LIBRARY</p>
                  <h3 id="profile-dialog-title">Manage profiles</h3>
                </div>
                <button className="log-clear" onClick={() => setProfilesOpen(false)} aria-label="Close profile manager">Close</button>
              </header>
              <div className="profile-library">
                <div className="profile-library__heading">
                  <span>{profiles.length} saved</span>
                  <button className="button button--secondary" onClick={startNewProfile}>Add profile</button>
                </div>
                {profiles.length === 0 ? (
                  <p className="profile-library__empty">No profiles yet. Add a URI to create your first connection.</p>
                ) : profiles.map((profile) => (
                  <article className={`profile-row ${profile.id === selectedProfileId ? "profile-row--selected" : ""}`} key={profile.id}>
                    <button className="profile-row__select" onClick={() => setSelectedProfileId(profile.id)} disabled={busy || isRunning}>
                      <strong>{profile.name}</strong>
                      <span>TAG · {profile.tag}</span>
                    </button>
                    <button className="profile-row__action" onClick={() => startEditProfile(profile)} disabled={busy || isRunning}>Edit</button>
                    <button className="profile-row__action profile-row__action--delete" onClick={() => removeProfile(profile)} disabled={busy || isRunning}>Delete</button>
                  </article>
                ))}
              </div>
              {editingProfileId && (
                <form className="profile-editor" onSubmit={(event) => { event.preventDefault(); saveProfile(); }}>
                  <h4>{editingProfileId === "new" ? "New profile" : "Edit profile"}</h4>
                  <label>
                    <span>Profile name</span>
                    <input value={editorName} onChange={(event) => setEditorName(event.target.value)} placeholder="Name shown in the app" autoFocus />
                  </label>
                  <label>
                    <span>Connection URI</span>
                    <textarea value={editorURI} onChange={(event) => setEditorURI(event.target.value)} placeholder="vless://, vmess://, trojan://, or ss://" spellCheck={false} />
                  </label>
                  <div className="profile-editor__actions">
                    <button type="button" className="button button--secondary" onClick={pasteEditorURI}>Paste URI</button>
                    <span className="profile-editor__tag">TAG · {profileTagFromURI(editorURI)}</span>
                    <button type="button" className="log-clear" onClick={() => setEditingProfileId(null)}>Cancel</button>
                    <button type="submit" className="button button--connect">Save profile</button>
                  </div>
                </form>
              )}
            </section>
          </div>
        )}
      </section>
    </main>
  );
}
