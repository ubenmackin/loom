import { create } from 'zustand'
import { User, AuthResponse } from '../types'

interface AuthState {
  user: User | null
  token: string | null
  isAuthenticated: boolean
  isAdmin: boolean
  login: (authResponse: AuthResponse) => void
  logout: () => void
  updateUser: (user: User) => void
}

function getInitialAuth(): { user: User | null; token: string | null; isAuthenticated: boolean; isAdmin: boolean } {
  if (typeof window === 'undefined') {
    return { user: null, token: null, isAuthenticated: false, isAdmin: false }
  }
  const token = localStorage.getItem('loom_auth_token')
  if (token) {
    try {
      const userStr = localStorage.getItem('loom_auth_user')
      const user = userStr ? JSON.parse(userStr) : null
      return {
        user,
        token,
        isAuthenticated: true,
        isAdmin: user?.role === 'admin',
      }
    } catch {
      return { user: null, token: null, isAuthenticated: false, isAdmin: false }
    }
  }
  return { user: null, token: null, isAuthenticated: false, isAdmin: false }
}

const initialAuth = getInitialAuth()

export const useAuthStore = create<AuthState>((set) => ({
  user: initialAuth.user,
  token: initialAuth.token,
  isAuthenticated: initialAuth.isAuthenticated,
  isAdmin: initialAuth.isAdmin,

  login: (authResponse: AuthResponse) => {
    localStorage.setItem('loom_auth_token', authResponse.token)
    localStorage.setItem('loom_auth_user', JSON.stringify(authResponse.user))
    set({
      user: authResponse.user,
      token: authResponse.token,
      isAuthenticated: true,
      isAdmin: authResponse.user.role === 'admin',
    })
  },

  logout: () => {
    localStorage.removeItem('loom_auth_token')
    localStorage.removeItem('loom_auth_user')
    set({
      user: null,
      token: null,
      isAuthenticated: false,
      isAdmin: false,
    })
  },

  updateUser: (user: User) => {
    localStorage.setItem('loom_auth_user', JSON.stringify(user))
    set({ user, isAdmin: user.role === 'admin' })
  },
}))