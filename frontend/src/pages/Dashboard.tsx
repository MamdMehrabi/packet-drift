import { useState, useEffect } from "react";
import { Ping, GetConnectionStatus } from "../../wailsjs/go/main/App";
import { useAppStore } from "../store/app";

export function Dashboard() {
  const { status, setStatus, connectionStatus, setConnectionStatus } = useAppStore();
  const [response, setResponse] = useState<string>("");

  async function handlePing() {
    try {
      const result = await Ping();
      setResponse(result);
      setStatus("connected");
    } catch (err) {
      setResponse(String(err));
      setStatus("error");
    }
  }
  
  useEffect(() => {
    async function fetchStatus() {
      try {
        const st = await GetConnectionStatus();
        setConnectionStatus(st);
      } catch (e) {
        setConnectionStatus("unknown");
      }
    }
    fetchStatus();
  }, []);

  return (
    <main>
      <h1>Packet Drift</h1>
      <p>Status: {status}</p>
      <p>Connection status: {connectionStatus}</p>
      <button onClick={handlePing}>Ping backend</button>
      {response && <p>Response: {response}</p>}
    </main>
  );
}
