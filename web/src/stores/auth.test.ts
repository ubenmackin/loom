import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useAuthStore } from './auth'
import type { User, AuthResponse } from '../types'

// ── Helpers ─────────────────────────────────────────────────────────────────

const defaultState = {
  user: null,
  token: null,
  isAuthenticated: false,
  isAdmin: false,
}

const adminUser: User = {
  id: 'admin-1',
  username: 'admin',
  email: 'admin@example.com',
  display_name: 'Admin User',
  role: 'admin',
  created_at: '2025-01-01T00:00:00Z',
}

const normalUser: User = {
  id: 'user-1',
  username: 'user',
  email: 'user@example.com',
  display_name: 'Normal User',
  role: 'normal',
  created_at: '2025-01-01T00:00:00Z',
}

const adminAuthResponse: AuthResponse = {
  user: adminUser,
  token: 'admin-token-abc',
}

const normalAuthResponse: AuthResponse = {
  user: normalUser,
  token: 'normal-token-xyz',
}

// ── Tests ───────────────────────────────────────────────────────────────────

describe('useAuthStore', () => {
  beforeEach(() => {
    localStorage.clear()
    useAuthStore.setState(defaultState)
  })

  describe('initial state', () => {
    it('is not authenticated when no token is in localStorage', () => {
      const state = useAuthStore.getState()
      expect(state.user).toBeNull()
      expect(state.token).toBeNull()
      expect(state.isAuthenticated).toBe(false)
      expect(state.isAdmin).toBe(false)
    })
  })

  describe('login', () => {
    it('sets user, token, isAuthenticated=true, and isAdmin=true for admin users', () => {
      useAuthStore.getState().login(adminAuthResponse)

      const state = useAuthStore.getState()
      expect(state.user).toEqual(adminUser)
      expect(state.token).toBe('admin-token-abc')
      expect(state.isAuthenticated).toBe(true)
      expect(state.isAdmin).toBe(true)
    })

    it('sets isAdmin=false for normal users', () => {
      useAuthStore.getState().login(normalAuthResponse)

      const state = useAuthStore.getState()
      expect(state.user).toEqual(normalUser)
      expect(state.token).toBe('normal-token-xyz')
      expect(state.isAuthenticated).toBe(true)
      expect(state.isAdmin).toBe(false)
    })

    it('stores token and user in localStorage', () => {
      useAuthStore.getState().login(normalAuthResponse)

      expect(localStorage.getItem('loom_auth_token')).toBe('normal-token-xyz')
      expect(localStorage.getItem('loom_auth_user')).toBe(JSON.stringify(normalUser))
    })
  })

  describe('logout', () => {
    it('clears all state (user, token, isAuthenticated, isAdmin)', () => {
      useAuthStore.getState().login(adminAuthResponse)
      expect(useAuthStore.getState().isAuthenticated).toBe(true)

      useAuthStore.getState().logout()

      const state = useAuthStore.getState()
      expect(state.user).toBeNull()
      expect(state.token).toBeNull()
      expect(state.isAuthenticated).toBe(false)
      expect(state.isAdmin).toBe(false)
    })

    it('removes token and user from localStorage', () => {
      useAuthStore.getState().login(normalAuthResponse)
      expect(localStorage.getItem('loom_auth_token')).not.toBeNull()

      useAuthStore.getState().logout()

      expect(localStorage.getItem('loom_auth_token')).toBeNull()
      expect(localStorage.getItem('loom_auth_user')).toBeNull()
    })
  })

  describe('updateUser', () => {
    it('updates user and re-evaluates isAdmin', () => {
      useAuthStore.getState().login(adminAuthResponse)
      expect(useAuthStore.getState().isAdmin).toBe(true)

      useAuthStore.getState().updateUser(normalUser)

      const state = useAuthStore.getState()
      expect(state.user).toEqual(normalUser)
      expect(state.isAdmin).toBe(false)
      // Token and authenticated status remain unchanged
      expect(state.token).toBe('admin-token-abc')
      expect(state.isAuthenticated).toBe(true)
    })

    it('updates localStorage with the new user data', () => {
      useAuthStore.getState().login(adminAuthResponse)

      const updatedUser: User = { ...normalUser, display_name: 'Updated Name' }
      useAuthStore.getState().updateUser(updatedUser)

      expect(localStorage.getItem('loom_auth_user')).toBe(JSON.stringify(updatedUser))
    })

    it('sets isAdmin=true when updated user has admin role', () => {
      useAuthStore.getState().login(normalAuthResponse)
      expect(useAuthStore.getState().isAdmin).toBe(false)

      useAuthStore.getState().updateUser(adminUser)

      expect(useAuthStore.getState().isAdmin).toBe(true)
    })
  })

  describe('SSR safety', () => {
    afterEach(() => {
      vi.unstubAllGlobals()
    })

    it('returns unauthenticated initial state when window is undefined', async () => {
      vi.stubGlobal('window', undefined)

      vi.resetModules()
      const { useAuthStore: freshStore } = await import('./auth')

      expect(freshStore.getState().user).toBeNull()
      expect(freshStore.getState().token).toBeNull()
      expect(freshStore.getState().isAuthenticated).toBe(false)
      expect(freshStore.getState().isAdmin).toBe(false)
    })

    it('provides unauthenticated initial state when no tokens exist in storage', () => {
      // localStorage is empty (cleared in beforeEach), so even when running
      // in a browser-like environment, the store initializes as unauthenticated.
      // This mirrors the SSR code path where `typeof window === 'undefined'`
      // and no tokens are available.
      const state = useAuthStore.getState()
      expect(state.user).toBeNull()
      expect(state.token).toBeNull()
      expect(state.isAuthenticated).toBe(false)
      expect(state.isAdmin).toBe(false)
    })

    it('returns unauthenticated state when localStorage has a valid token but corrupt user JSON', async () => {
      // Arrange: set a valid token but an invalid user JSON so JSON.parse fails
      localStorage.setItem('loom_auth_token', 'some-valid-token')
      localStorage.setItem('loom_auth_user', '{bad-json}')

      // Force module re-evaluation so getInitialAuth() picks up the corrupt data
      vi.resetModules()
      const { useAuthStore: freshStore } = await import('./auth')

      // Assert: the catch block returns the fallback unauthenticated state
      expect(freshStore.getState().user).toBeNull()
      expect(freshStore.getState().token).toBeNull()
      expect(freshStore.getState().isAuthenticated).toBe(false)
      expect(freshStore.getState().isAdmin).toBe(false)
    })
  })
})
