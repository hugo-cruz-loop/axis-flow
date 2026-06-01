import { create } from 'zustand'
import type { User } from '@/types/users'

// Access token is stored in memory (Zustand), NOT in localStorage.
// Storing tokens in localStorage exposes them to XSS attacks via malicious scripts.
// Memory-based storage clears on page refresh, which is an intentional security tradeoff.

interface AuthState {
  accessToken: string | null
  user: User | null
  isAuthenticated: boolean
  setAuth: (token: string, user: User) => void
  clearAuth: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: null,
  user: null,
  isAuthenticated: false,
  setAuth: (token, user) =>
    set({ accessToken: token, user, isAuthenticated: true }),
  clearAuth: () =>
    set({ accessToken: null, user: null, isAuthenticated: false }),
}))
