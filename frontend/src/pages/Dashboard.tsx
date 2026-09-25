import { useState } from "react";
import { Ping } from "../../wailsjs/go/main/App";
import { useAppStore } from "../store/app";

export function Dashboard() {
  const { status, setStatus } = useAppStore();
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

  return (
    <main>
      <h1>Packet Drift</h1>
      <p>Status: {status}</p>
      <button onClick={handlePing}>Ping backend</button>
      {response && <p>Response: {response}</p>}
    </main>
  );
}
