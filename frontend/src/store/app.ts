import { create } from "zustand";

interface AppState {
  status: string;
  setStatus: (status: string) => void;
}

export const useAppStore = create<AppState>((set) => ({
  status: "idle",
  setStatus: (status) => set({ status }),
}));
