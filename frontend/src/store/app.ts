import { create } from "zustand";

interface AppState {
  connectionStatus: string;
  setConnectionStatus: (status: string) => void;
}

export const useAppStore = create<AppState>((set) => ({
  connectionStatus: "unknown",
  setConnectionStatus: (status) => set({ connectionStatus: status }),
}));
