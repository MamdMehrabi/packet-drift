import { useState, useEffect, useRef } from "react";
import {
  GetClipboardText,
  GetConnectionStatus,
  GetTrafficStats,
  GetXrayBinaryPath,
  StartXrayWithConfig,
  StopXray,
} from "../../wailsjs/go/main/App";
import { useAppStore } from "../store/app";

interface TrafficStats {
  uploadBytes: number;
  downloadBytes: number;
  uploadBytesPerSecond: number;
  downloadBytesPerSecond: number;
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
  const [configJSON, setConfigJSON] = useState("");
  const [binaryPath, setBinaryPath] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [traffic, setTraffic] = useState<TrafficStats>(emptyTraffic);
  const [history, setHistory] = useState<TrafficStats[]>([]);
  const [statsUnavailable, setStatsUnavailable] = useState(false);
  const refreshInFlight = useRef(false);

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

  async function handlePasteConfig() {
    setError("");
    try {
      const clipboard = await GetClipboardText();
      if (!clipboard.trim()) {
        setError("The clipboard is empty.");
        return;
      }
      setConfigJSON(clipboard.trim());
    } catch (err) {
      setError(String(err));
    }
  }

  async function handleConnect() {
    if (!configJSON.trim()) {
      setError("Paste an Xray JSON configuration first.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await StartXrayWithConfig(binaryPath, configJSON);
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
          <h1 id="app-title">Packet Drift</h1>
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
            {isRunning ? "The local proxy engine is running." : "Paste a configuration to start the engine."}
          </p>
        </section>

        <section className="traffic" aria-label="Traffic dashboard">
          <article className="traffic-panel">
            <div className="traffic-panel__heading">
              <span className="traffic-panel__label">Download</span>
              <strong>{isRunning ? formatRate(traffic.downloadBytesPerSecond) : "--"}</strong>
            </div>
            <div className="traffic-chart traffic-chart--download" role="img" aria-label="Download speed history">
              {history.map((sample, index) => (
                <span key={index} style={{ height: `${Math.max(4, (sample.downloadBytesPerSecond / maxDownload) * 100)}%` }} />
              ))}
            </div>
            <span className="traffic-panel__total">{formatBytes(traffic.downloadBytes)} received</span>
          </article>
          <article className="traffic-panel">
            <div className="traffic-panel__heading">
              <span className="traffic-panel__label">Upload</span>
              <strong>{isRunning ? formatRate(traffic.uploadBytesPerSecond) : "--"}</strong>
            </div>
            <div className="traffic-chart traffic-chart--upload" role="img" aria-label="Upload speed history">
              {history.map((sample, index) => (
                <span key={index} style={{ height: `${Math.max(4, (sample.uploadBytesPerSecond / maxUpload) * 100)}%` }} />
              ))}
            </div>
            <span className="traffic-panel__total">{formatBytes(traffic.uploadBytes)} sent</span>
          </article>
        </section>

        {statsUnavailable && isRunning && <p className="stats-note">Waiting for Xray traffic statistics...</p>}

        <section className="config-section" aria-labelledby="config-title">
          <div className="config-heading">
            <div>
              <p className="eyebrow">CONNECTION PROFILE</p>
              <h3 id="config-title">Xray JSON</h3>
            </div>
            <button className="button button--secondary" onClick={handlePasteConfig} disabled={busy || isRunning}>
              Paste from clipboard
            </button>
          </div>
          <textarea
            aria-label="Xray JSON configuration"
            className="config-input"
            value={configJSON}
            onChange={(event) => setConfigJSON(event.target.value)}
            placeholder={'Paste a complete Xray JSON config here\n{\n  "inbounds": [],\n  "outbounds": []\n}'}
            spellCheck={false}
            disabled={busy || isRunning}
          />
          <div className="config-meta">
            <span>{binaryPath ? "Xray 26.3.27 bundled" : "Locating bundled Xray..."}</span>
            <span>{configJSON.length.toLocaleString()} characters</span>
          </div>
        </section>

        {error && <p className="error" role="alert">{error}</p>}

        <footer className="actions">
          {isRunning ? (
            <button className="button button--disconnect" onClick={handleDisconnect} disabled={busy}>
              {busy ? "Working..." : "Disconnect"}
            </button>
          ) : (
            <button className="button button--connect" onClick={handleConnect} disabled={busy || !configJSON.trim()}>
              {busy ? "Connecting..." : "Connect"}
            </button>
          )}
        </footer>
      </section>
    </main>
  );
}
