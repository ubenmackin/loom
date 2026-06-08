import { describe, it, expect, beforeEach } from 'vitest'
import { useProjectFilterStore } from './project'
import { useAuthStore } from './auth'
import type { User } from '../types'

const normalUser: User = {
  id: 'user-123',
  username: 'alice',
  email: 'alice@example.com',
  display_name: 'Alice',
  role: 'normal',
  created_at: '2025-01-01T00:00:00Z',
}

const otherUser: User = {
  id: 'user-456',
  username: 'bob',
  email: 'bob@example.com',
  display_name: 'Bob',
  role: 'normal',
  created_at: '2025-01-01T00:00:00Z',
}

describe('useProjectFilterStore', () => {
  beforeEach(() => {
    localStorage.clear()
    useAuthStore.setState({ user: null, token: null, isAuthenticated: false })
    useProjectFilterStore.setState({ selectedProjectId: null })
  })

  describe('initial state', () => {
    it('is null when no user is logged in', () => {
      expect(useProjectFilterStore.getState().selectedProjectId).toBeNull()
    })

    it('loads the correct project when a user is logged in and has a saved project', async () => {
      // Set up saved project in localStorage first
      localStorage.setItem('loom_selected_project_user-123', 'proj-xyz')
      
      // Simulate auth store logged in
      useAuthStore.setState({ user: normalUser, token: 'token-abc', isAuthenticated: true })

      // Force a module reset / re-evaluation or trigger the subscriber manually by updating the state
      // Since the store was already initialized, we can trigger the subscriber by changing user ID
      useAuthStore.setState({ user: { ...normalUser, id: 'user-123' } })

      expect(useProjectFilterStore.getState().selectedProjectId).toBe('proj-xyz')
    })
  })

  describe('setSelectedProjectId', () => {
    it('updates state but does not write to localStorage if no user is logged in', () => {
      useProjectFilterStore.getState().setSelectedProjectId('proj-1')
      expect(useProjectFilterStore.getState().selectedProjectId).toBe('proj-1')
      expect(localStorage.getItem('loom_selected_project_user-123')).toBeNull()
    })

    it('updates state and writes to localStorage under user ID when user is logged in', () => {
      useAuthStore.setState({ user: normalUser, token: 'token', isAuthenticated: true })

      useProjectFilterStore.getState().setSelectedProjectId('proj-2')
      expect(useProjectFilterStore.getState().selectedProjectId).toBe('proj-2')
      expect(localStorage.getItem('loom_selected_project_user-123')).toBe('proj-2')
    })

    it('removes item from localStorage when setting project ID to null', () => {
      useAuthStore.setState({ user: normalUser, token: 'token', isAuthenticated: true })
      localStorage.setItem('loom_selected_project_user-123', 'proj-2')

      useProjectFilterStore.getState().setSelectedProjectId(null)
      expect(useProjectFilterStore.getState().selectedProjectId).toBeNull()
      expect(localStorage.getItem('loom_selected_project_user-123')).toBeNull()
    })
  })

  describe('clearProjectFilter', () => {
    it('sets selectedProjectId to null and removes from localStorage', () => {
      useAuthStore.setState({ user: normalUser, token: 'token', isAuthenticated: true })
      useProjectFilterStore.getState().setSelectedProjectId('proj-9')
      expect(localStorage.getItem('loom_selected_project_user-123')).toBe('proj-9')

      useProjectFilterStore.getState().clearProjectFilter()
      expect(useProjectFilterStore.getState().selectedProjectId).toBeNull()
      expect(localStorage.getItem('loom_selected_project_user-123')).toBeNull()
    })
  })

  describe('auth subscription', () => {
    it('loads user-specific project when user logs in', () => {
      localStorage.setItem('loom_selected_project_user-123', 'proj-alice')
      localStorage.setItem('loom_selected_project_user-456', 'proj-bob')

      // 1. Login Alice
      useAuthStore.setState({ user: normalUser, token: 'token-alice', isAuthenticated: true })
      expect(useProjectFilterStore.getState().selectedProjectId).toBe('proj-alice')

      // 2. Login Bob
      useAuthStore.setState({ user: otherUser, token: 'token-bob', isAuthenticated: true })
      expect(useProjectFilterStore.getState().selectedProjectId).toBe('proj-bob')
    })

    it('resets project to null when user logs out', () => {
      useAuthStore.setState({ user: normalUser, token: 'token-alice', isAuthenticated: true })
      useProjectFilterStore.getState().setSelectedProjectId('proj-alice')

      // Logout
      useAuthStore.setState({ user: null, token: null, isAuthenticated: false })
      expect(useProjectFilterStore.getState().selectedProjectId).toBeNull()
    })
  })
})
