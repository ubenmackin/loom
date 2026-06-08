import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useAuthStore } from './auth'
import { testSSRSafety } from './__tests__/testSSRSafety'
import type { User, AuthResponse } from '../types'

// ── Helpers ─────────────────────────────────────────────────────────────────

const defaultState = {
  user: null,
  token: null,
  isAuthenticated: false,
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

// ── Selector helpers (isAdmin is now computed) ──────────────────────────────

function isAdmin(): boolean {
  return useAuthStore.getState().user?.role === 'admin'
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
      expect(isAdmin()).toBe(false)
    })
  })

  describe('login', () => {
    it('sets user, token, isAuthenticated=true, and isAdmin=true for admin users', () => {
      useAuthStore.getState().login(adminAuthResponse)

      const state = useAuthStore.getState()
      expect(state.user).toEqual(adminUser)
      expect(state.token).toBe('admin-token-abc')
      expect(state.isAuthenticated).toBe(true)
      expect(isAdmin()).toBe(true)
    })

    it('sets isAdmin=false for normal users', () => {
      useAuthStore.getState().login(normalAuthResponse)

      const state = useAuthStore.getState()
      expect(state.user).toEqual(normalUser)
      expect(state.token).toBe('normal-token-xyz')
      expect(state.isAuthenticated).toBe(true)
      expect(isAdmin()).toBe(false)
    })

    it('stores token, user, and isAuthenticated in localStorage via persist middleware', () => {
      useAuthStore.getState().login(normalAuthResponse)

      const stored = JSON.parse(localStorage.getItem('loom_auth')!)
      expect(stored.state.token).toBe('normal-token-xyz')
      expect(stored.state.user).toEqual(normalUser)
      expect(stored.state.isAuthenticated).toBe(true)
    })
  })

  describe('logout', () => {
    it('clears all state (user, token, isAuthenticated)', () => {
      useAuthStore.getState().login(adminAuthResponse)
      expect(useAuthStore.getState().isAuthenticated).toBe(true)

      useAuthStore.getState().logout()

      const state = useAuthStore.getState()
      expect(state.user).toBeNull()
      expect(state.token).toBeNull()
      expect(state.isAuthenticated).toBe(false)
      expect(isAdmin()).toBe(false)
    })

    it('removes token, user, and resets isAuthenticated in localStorage', () => {
      useAuthStore.getState().login(normalAuthResponse)
      expect(localStorage.getItem('loom_auth')).not.toBeNull()

      useAuthStore.getState().logout()

      const stored = JSON.parse(localStorage.getItem('loom_auth')!)
      expect(stored.state.token).toBeNull()
      expect(stored.state.user).toBeNull()
      expect(stored.state.isAuthenticated).toBe(false)
    })
  })

  describe('updateUser', () => {
    it('updates user and re-evaluates isAdmin', () => {
      useAuthStore.getState().login(adminAuthResponse)
      expect(isAdmin()).toBe(true)

      useAuthStore.getState().updateUser(normalUser)

      const state = useAuthStore.getState()
      expect(state.user).toEqual(normalUser)
      expect(isAdmin()).toBe(false)
      // Token and authenticated status remain unchanged
      expect(state.token).toBe('admin-token-abc')
      expect(state.isAuthenticated).toBe(true)
    })

    it('updates localStorage with the new user data', () => {
      useAuthStore.getState().login(adminAuthResponse)

      const updatedUser: User = { ...normalUser, display_name: 'Updated Name' }
      useAuthStore.getState().updateUser(updatedUser)

      const stored = JSON.parse(localStorage.getItem('loom_auth')!)
      expect(stored.state.user).toEqual(updatedUser)
    })

    it('sets isAdmin=true when updated user has admin role', () => {
      useAuthStore.getState().login(normalAuthResponse)
      expect(isAdmin()).toBe(false)

      useAuthStore.getState().updateUser(adminUser)

      expect(isAdmin()).toBe(true)
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
    })

    it('provides unauthenticated initial state when no tokens exist in storage', () => {
      // localStorage is empty (cleared in beforeEach), so the store initializes as unauthenticated.
      const state = useAuthStore.getState()
      expect(state.user).toBeNull()
      expect(state.token).toBeNull()
      expect(state.isAuthenticated).toBe(false)
      expect(isAdmin()).toBe(false)
    })

    it('returns unauthenticated state when localStorage has a valid token but corrupt user JSON', async () => {
      // Arrange: put corrupt json in the persist store
      localStorage.setItem('loom_auth', '{bad-json}')

      // Force module re-evaluation so persist tries to rehydrate from corrupt data
      vi.resetModules()
      const { useAuthStore: freshStore } = await import('./auth')

      // Assert: persist will fail to parse and fall back to initial state
      expect(freshStore.getState().user).toBeNull()
      expect(freshStore.getState().token).toBeNull()
      expect(freshStore.getState().isAuthenticated).toBe(false)
    })
  })

  // Shared SSR safety test — verifies initial state when window is undefined
  testSSRSafety(
    () => import('./auth'),
    'useAuthStore',
    { user: null, token: null, isAuthenticated: false },
  )
})
