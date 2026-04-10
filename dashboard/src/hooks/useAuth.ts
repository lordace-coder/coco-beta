import { create } from "zustand";
import { type Admin } from "@/api/client";

interface AuthState {
  token: string | null;
  admin: Admin | null;
  setAuth: (token: string, admin: Admin) => void;
  logout: () => void;
}

// Simple zustand store — we'll use localStorage for persistence
export const useAuthStore = create<AuthState>((set) => ({
  token: localStorage.getItem("admin_token"),
  admin: null,
  setAuth: (token, admin) => {
    localStorage.setItem("admin_token", token);
    set({ token, admin });
  },
  logout: () => {
    localStorage.removeItem("admin_token");
    set({ token: null, admin: null });
    window.location.href = "/_/login";
  },
}));
