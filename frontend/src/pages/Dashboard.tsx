import { useState, useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import {
  GetClipboardText,
  GetConnectionStatus,
  GetTrafficStats,
  GetXrayBinaryPath,
  GetLANAddresses,
  FetchSubscription,
  PingServer,
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

type ConnectionMode = "proxy" | "lan" | "tunnel";

interface ConnectionNode {
  id: string;
  name: string;
  uri: string;
}

interface ConnectionProfile {
  id: string;
  name: string;
  subscriptionURL: string;
  nodes: ConnectionNode[];
  selectedNodeId: string;
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
    return parsed.flatMap((profile): ConnectionProfile[] => {
      if (typeof profile?.id !== "string" || typeof profile?.name !== "string") return [];
      const nodes: ConnectionNode[] = Array.isArray(profile.nodes)
        ? profile.nodes.filter((node: ConnectionNode) =>
          typeof node?.id === "string" && typeof node?.name === "string" && typeof node?.uri === "string",
        )
        : [];
      if (nodes.length === 0 && typeof profile.uri === "string" && profile.uri.trim()) {
        nodes.push({
          id: `${profile.id}-legacy`,
          name: typeof profile.tag === "string" && profile.tag ? profile.tag : profileTagFromURI(profile.uri),
          uri: profile.uri,
        });
      }
      if (nodes.length === 0) return [];
      const selectedNodeId = nodes.some((node) => node.id === profile.selectedNodeId)
        ? profile.selectedNodeId
        : nodes[0].id;
      return [{
        id: profile.id,
        name: profile.name,
        subscriptionURL: typeof profile.subscriptionURL === "string" ? profile.subscriptionURL : "",
        nodes,
        selectedNodeId,
      }];
    });
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
  const [editorSubscriptionURL, setEditorSubscriptionURL] = useState("");
  const [editorNodes, setEditorNodes] = useState<ConnectionNode[]>([]);
  const [fetchingSubscription, setFetchingSubscription] = useState(false);
  const [connectionMode, setConnectionMode] = useState<ConnectionMode>(() =>
    storedSetting("packet-drift-mode", "proxy") === "tunnel"
      ? "tunnel"
      : storedSetting("packet-drift-mode", "proxy") === "lan" ? "lan" : "proxy",
  );
  const [proxyAddress, setProxyAddress] = useState(() =>
    storedSetting("packet-drift-address", "127.0.0.1:10808"),
  );
  const [lanAddresses, setLanAddresses] = useState<string[]>([]);
  const [lanBindAddress, setLanBindAddress] = useState(() => storedSetting("packet-drift-lan-address", ""));
  const [lanPort, setLanPort] = useState(() => storedSetting("packet-drift-lan-port", "10808"));
  const [binaryPath, setBinaryPath] = useState("");
  const [startupVisible, setStartupVisible] = useState(true);
  const [startupLeaving, setStartupLeaving] = useState(false);
  const [busy, setBusy] = useState(false);
  const [pingingNodeId, setPingingNodeId] = useState<string | null>(null);
  const [pingResult, setPingResult] = useState<{ nodeId: string; latency: number | null } | null>(null);
  const [error, setError] = useState("");
  const [traffic, setTraffic] = useState<TrafficStats>(emptyTraffic);
  const [history, setHistory] = useState<TrafficStats[]>([]);
  const [logs, setLogs] = useState<string[]>([]);
  const [statsUnavailable, setStatsUnavailable] = useState(false);
  const refreshInFlight = useRef(false);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const selectedProfile = profiles.find((profile) => profile.id === selectedProfileId) ?? null;
  const selectedNode = selectedProfile
    ? selectedProfile.nodes.find((node) => node.id === selectedProfile.selectedNodeId) ?? null
    : null;
  const proxyURI = selectedNode?.uri ?? "";

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
    let startupTimer = 0;
    async function initialize() {
      const startupStartedAt = performance.now();
      try {
        const [pathResult, , lanResult] = await Promise.allSettled([
          GetXrayBinaryPath().then((path) => {
            if (active) setBinaryPath(path);
          }),
          refreshDashboard(),
          GetLANAddresses(),
        ]);
        if (pathResult.status === "rejected" && active) setError(String(pathResult.reason));
        if (active && lanResult.status === "fulfilled") {
          setLanAddresses(lanResult.value);
          setLanBindAddress((current) => lanResult.value.includes(current) ? current : lanResult.value[0] ?? "");
        }
      } finally {
        if (active) {
          const visibleFor = performance.now() - startupStartedAt;
          const minimumDisplayDelay = Math.max(0, 820 - visibleFor);
          startupTimer = window.setTimeout(() => {
            setStartupLeaving(true);
            startupTimer = window.setTimeout(() => setStartupVisible(false), 620);
          }, minimumDisplayDelay);
        }
      }
    }
    void initialize();
    const timer = window.setInterval(() => {
      if (active) void refreshDashboard();
    }, 1000);
    return () => {
      active = false;
      window.clearInterval(timer);
      window.clearTimeout(startupTimer);
    };
  }, []);

  useEffect(() => {
    try {
      window.localStorage.setItem(profilesStorageKey, JSON.stringify(profiles));
      window.localStorage.setItem(selectedProfileStorageKey, selectedProfileId);
      window.localStorage.setItem("packet-drift-mode", connectionMode);
      window.localStorage.setItem("packet-drift-address", proxyAddress);
      window.localStorage.setItem("packet-drift-lan-address", lanBindAddress);
      window.localStorage.setItem("packet-drift-lan-port", lanPort);
    } catch {}
  }, [profiles, selectedProfileId, connectionMode, proxyAddress, lanBindAddress, lanPort]);

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
    setError("");
    setEditorName("");
    setEditorURI("");
    setEditorSubscriptionURL("");
    setEditorNodes([]);
    setEditingProfileId("new");
  }

  function startEditProfile(profile: ConnectionProfile) {
    setError("");
    setEditorName(profile.name);
    setEditorURI("");
    setEditorSubscriptionURL(profile.subscriptionURL);
    setEditorNodes(profile.nodes);
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

  async function fetchEditorSubscription() {
    setError("");
    setFetchingSubscription(true);
    try {
      const imported = await FetchSubscription(editorSubscriptionURL.trim());
      setEditorNodes(imported.map((node) => ({ id: crypto.randomUUID(), name: node.name, uri: node.uri })));
      if (!editorName.trim()) {
        try {
          setEditorName(new URL(editorSubscriptionURL).hostname);
        } catch {
          setEditorName("Subscription");
        }
      }
    } catch (err) {
      setError(String(err));
    } finally {
      setFetchingSubscription(false);
    }
  }

  function addEditorURI() {
    const uri = editorURI.trim();
    if (!uri) return;
    if (!/^(vless|vmess|trojan|ss):\/\//i.test(uri)) {
      setError("Enter a supported proxy URI.");
      return;
    }
    setEditorNodes((nodes) => [...nodes, { id: crypto.randomUUID(), name: profileTagFromURI(uri), uri }]);
    setEditorURI("");
    setError("");
  }

  function saveProfile() {
    const nodes = [...editorNodes];
    const uri = editorURI.trim();
    if (uri) {
      if (!/^(vless|vmess|trojan|ss):\/\//i.test(uri)) {
        setError("Enter a supported proxy URI.");
        return;
      }
      nodes.push({ id: crypto.randomUUID(), name: profileTagFromURI(uri), uri });
    }
    if (nodes.length === 0) {
      setError("Fetch a subscription or add at least one proxy URI.");
      return;
    }
    const id = editingProfileId === "new" ? crypto.randomUUID() : editingProfileId;
    if (!id) return;
    const previousProfile = profiles.find((item) => item.id === id);
    const name = editorName.trim() || editorSubscriptionURL.trim() || nodes[0].name;
    const profile: ConnectionProfile = {
      id,
      name,
      subscriptionURL: editorSubscriptionURL.trim(),
      nodes,
      selectedNodeId: nodes.some((node) => node.id === previousProfile?.selectedNodeId)
        ? previousProfile?.selectedNodeId ?? nodes[0].id
        : nodes[0].id,
    };
    setProfiles((current) => {
      const index = current.findIndex((item) => item.id === id);
      if (index < 0) return [...current, profile];
      return current.map((item) => item.id === id ? profile : item);
    });
    setSelectedProfileId(id);
    setEditingProfileId(null);
    setError("");
  }

  function selectProfileNode(profile: ConnectionProfile, nodeId: string) {
    setSelectedProfileId(profile.id);
    setProfiles((current) => current.map((item) => item.id === profile.id ? { ...item, selectedNodeId: nodeId } : item));
    setPingResult(null);
  }

  function selectConnectionMode(mode: ConnectionMode) {
    setConnectionMode(mode);
    if (mode === "lan" && !lanBindAddress && lanAddresses.length > 0) {
      setLanBindAddress(lanAddresses[0]);
    }
  }

  async function handlePingServer() {
    if (!selectedNode) return;
    setPingingNodeId(selectedNode.id);
    setPingResult(null);
    try {
      const latency = await PingServer(selectedNode.uri);
      setPingResult({ nodeId: selectedNode.id, latency });
    } catch {
      setPingResult({ nodeId: selectedNode.id, latency: null });
    } finally {
      setPingingNodeId(null);
    }
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
      const listenAddress = connectionMode === "lan" ? `${lanBindAddress}:${lanPort}` : proxyAddress;
      await StartXrayWithURI(binaryPath, proxyURI, connectionMode, listenAddress);
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
    <main aria-busy={startupVisible}>
      <section className="dashboard" aria-labelledby="app-title">
        <header className="topbar">
          <div className="brand-block">
            <div className="brand-signature">
              <h1 id="app-title">Packet Drift</h1>
              <p className="developer-credit">
                Developed by <strong>themamd</strong>
                <span aria-hidden="true"> · </span>
                <a href="https://t.me/TheCommit" target="_blank" rel="noreferrer" aria-label="Telegram channel TheCommit">
                  Telegram @TheCommit
                </a>
              </p>
            </div>
            <div className="selected-profile" title={selectedNode ? `Server: ${selectedNode.name}` : "No profile selected"}>
              <span>{selectedProfile?.name ?? "No profile selected"}</span>
              {selectedNode && <small>SERVER · {selectedNode.name}</small>}
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
              ? connectionMode === "lan"
                ? `LAN SOCKS5 · ${lanBindAddress}:${lanPort}`
                : `${connectionMode === "proxy" ? "Proxy" : "Tunnel"} mode is active.`
              : selectedProfile
                ? "Your server is ready to connect."
                : "Add a profile to get started."}
          </p>
        </section>

        {isRunning && <section className="traffic" aria-label="Profile outbound traffic since connection">
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
        </section>}

        {statsUnavailable && isRunning && <p className="stats-note">Waiting for Xray traffic statistics...</p>}

        <section className="config-section" aria-labelledby="config-title">
          <div className="config-heading">
            <div>
              <p className="eyebrow">CONNECTION PROFILE</p>
              <h3 id="config-title">Profiles</h3>
            </div>
            {profiles.length > 0 && (
              <button className="button button--secondary" onClick={openProfileManager} disabled={busy || isRunning}>
                Manage
              </button>
            )}
          </div>
          {profiles.length > 0 ? (
            <>
              <label className="profile-select-label" htmlFor="profile-select">Profile</label>
              <select
                id="profile-select"
                className="profile-select"
                value={selectedProfileId}
                onChange={(event) => setSelectedProfileId(event.target.value)}
                disabled={busy || isRunning}
              >
                {profiles.map((profile) => <option key={profile.id} value={profile.id}>{profile.name} · {profile.nodes.length} servers</option>)}
              </select>
              <label className="profile-select-label" htmlFor="node-select">Server</label>
              <div className="server-check">
            <select
              id="node-select"
              className="profile-select"
              value={selectedNode?.id ?? ""}
              onChange={(event) => selectedProfile && selectProfileNode(selectedProfile, event.target.value)}
              disabled={busy || isRunning || !selectedProfile}
            >
              {!selectedProfile && <option value="">No profile selected</option>}
              {selectedProfile?.nodes.map((node) => <option key={node.id} value={node.id}>{node.name}</option>)}
            </select>
            <button
              type="button"
              className="button button--secondary ping-button"
              onClick={handlePingServer}
              disabled={!selectedNode || pingingNodeId !== null}
            >
              {pingingNodeId === selectedNode?.id ? "Testing..." : "Ping server"}
            </button>
              </div>
              {pingResult && selectedNode && pingResult.nodeId === selectedNode.id && (
                <p className={`ping-result ${pingResult.latency === null ? "ping-result--failed" : "ping-result--success"}`} role="status">
                  {pingResult.latency === null ? "Server is unreachable" : `TCP reachable · ${pingResult.latency} ms`}
                </p>
              )}
            </>
          ) : (
            <p className="profile-empty">No profiles yet</p>
          )}
        </section>

        <details className="settings-section">
          <summary className="settings-summary">
            <span>Connection settings</span>
            {binaryPath && <span className="settings-runtime">Xray 26.3.27</span>}
          </summary>
          <div className="settings-grid">
            <fieldset className="mode-field" disabled={busy || isRunning}>
              <legend>Connection mode</legend>
              <div className="segmented-control">
                <button
                  type="button"
                  className={connectionMode === "proxy" ? "is-selected" : ""}
                  aria-pressed={connectionMode === "proxy"}
                  onClick={() => selectConnectionMode("proxy")}
                >
                  Proxy
                </button>
                <button
                  type="button"
                  className={connectionMode === "lan" ? "is-selected" : ""}
                  aria-pressed={connectionMode === "lan"}
                  onClick={() => selectConnectionMode("lan")}
                >
                  LAN
                </button>
                <button
                  type="button"
                  className={connectionMode === "tunnel" ? "is-selected" : ""}
                  aria-pressed={connectionMode === "tunnel"}
                  onClick={() => selectConnectionMode("tunnel")}
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
            ) : connectionMode === "lan" ? (
              <div className="lan-share-fields">
                <label className="address-field">
                  <span>LAN address</span>
                  <select
                    className="profile-select"
                    value={lanBindAddress}
                    onChange={(event) => setLanBindAddress(event.target.value)}
                    disabled={busy || isRunning || lanAddresses.length === 0}
                  >
                    {lanAddresses.length === 0 && <option value="">No private network found</option>}
                    {lanAddresses.map((address) => <option key={address} value={address}>{address}</option>)}
                  </select>
                </label>
                <label className="address-field">
                  <span>Port</span>
                  <input
                    type="number"
                    min="1"
                    max="65535"
                    value={lanPort}
                    onChange={(event) => setLanPort(event.target.value)}
                    disabled={busy || isRunning}
                  />
                </label>
                {lanBindAddress ? (
                  <p className="lan-note">
                    SOCKS5 has no password; use a trusted Wi-Fi only. On your phone, enter <strong>{lanBindAddress}:{lanPort}</strong> as the SOCKS5 server. Allow TCP port {lanPort} through the computer firewall; do not configure router port forwarding.
                  </p>
                ) : (
                  <p className="lan-note lan-note--warning">Connect this computer to a private Wi-Fi or LAN network to enable sharing.</p>
                )}
              </div>
            ) : (
              <p className="tunnel-note">Creates a TUN interface. System routing and network permissions are required.</p>
            )}
          </div>
        </details>

        {error && <p className="error" role="alert">{error}</p>}

        <footer className="actions">
          {isRunning ? (
            <button className="button button--disconnect" onClick={handleDisconnect} disabled={busy}>
              {busy ? "Working..." : "Disconnect"}
            </button>
          ) : selectedProfile ? (
            <button className="button button--connect" onClick={handleConnect} disabled={busy || (connectionMode === "lan" && (!lanBindAddress || !/^\d+$/.test(lanPort) || Number(lanPort) < 1 || Number(lanPort) > 65535))}>
              {busy ? "Connecting..." : "Connect"}
            </button>
          ) : (
            <button className="button button--connect" onClick={openProfileManager} disabled={busy}>
              Add a profile
            </button>
          )}
        </footer>

        <details className="log-section">
          <summary className="log-heading">
            <span id="logs-title">Live logs</span>
          </summary>
          <div className="log-tools">
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
        </details>

        {profilesOpen && createPortal((
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
                      <span>{profile.nodes.length} servers{profile.subscriptionURL ? ` · ${profile.subscriptionURL}` : " · manual"}</span>
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
                    <span>Subscription URL</span>
                    <input
                      value={editorSubscriptionURL}
                      onChange={(event) => setEditorSubscriptionURL(event.target.value)}
                      placeholder="https://provider.example/subscription"
                      spellCheck={false}
                    />
                  </label>
                  <div className="subscription-actions">
                    <span>{editorNodes.length} servers imported</span>
                    <button type="button" className="button button--secondary" onClick={fetchEditorSubscription} disabled={fetchingSubscription || !editorSubscriptionURL.trim()}>
                      {fetchingSubscription ? "Fetching..." : "Fetch subscription"}
                    </button>
                  </div>
                  {editorNodes.length > 0 && (
                    <ul className="editor-node-list" aria-label="Profile servers">
                      {editorNodes.map((node) => (
                        <li key={node.id}>
                          <span title={node.uri}>{node.name}</span>
                          <button type="button" className="profile-row__action profile-row__action--delete" onClick={() => setEditorNodes((nodes) => nodes.filter((item) => item.id !== node.id))} aria-label={`Remove ${node.name}`}>
                            Remove
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                  <label>
                    <span>Add a single server</span>
                    <textarea value={editorURI} onChange={(event) => setEditorURI(event.target.value)} placeholder="vless://, vmess://, trojan://, or ss://" spellCheck={false} />
                  </label>
                  <div className="profile-editor__actions">
                    <button type="button" className="button button--secondary" onClick={pasteEditorURI}>Paste URI</button>
                    <button type="button" className="button button--secondary" onClick={addEditorURI} disabled={!editorURI.trim()}>Add server</button>
                    <button type="button" className="log-clear" onClick={() => setEditingProfileId(null)}>Cancel</button>
                    <button type="submit" className="button button--connect">Save profile</button>
                  </div>
                  {error && <p className="profile-error" role="alert">{error}</p>}
                </form>
              )}
            </section>
          </div>
        ), document.body)}
      </section>
      {startupVisible && (
        <section
          className={`startup-screen ${startupLeaving ? "startup-screen--leaving" : ""}`}
          role="status"
          aria-live="polite"
          aria-label="Loading connection details"
        >
          <div className="startup-content">
            <div className="startup-mark" aria-hidden="true">
              <span />
              <span />
              <i />
            </div>
            <p className="startup-brand">Packet Drift</p>
            <p className="startup-message">Loading your connection details</p>
            <div className="startup-progress" aria-hidden="true"><span /></div>
          </div>
        </section>
      )}
    </main>
  );
}
