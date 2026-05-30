import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { User, AuthResponse } from '../types'

interface AuthState {
  user: User | null
  token: string | null
  isAuthenticated: boolean
  login: (authResponse: AuthResponse) => void
  logout: () => void
  updateUser: (user: User) => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      token: null,
      isAuthenticated: false,

      login: (authResponse: AuthResponse) => {
        set({
          user: authResponse.user,
          token: authResponse.token,
          isAuthenticated: true,
        })
      },

      logout: () => {
        set({
          user: null,
          token: null,
          isAuthenticated: false,
        })
      },

      updateUser: (user: User) => {
        set({ user })
      },
    }),
    {
      name: 'loom_auth',
      partialize: (state) => ({
        user: state.user,
        token: state.token,
      }),
    },
  ),
)