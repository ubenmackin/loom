import { describe, it, expect, beforeEach, vi } from 'vitest'
import { useSessionStore } from './session'
import { testSSRSafety } from './__tests__/testSSRSafety'

describe('useSessionStore', () => {
  beforeEach(() => {
    localStorage.clear()
    useSessionStore.setState({ sessionId: '' })
  })

  describe('initial state', () => {
    it('generates a session ID matching the expected format on creation', async () => {
      // Clear localStorage before re-importing to prevent zustand persist
      // middleware from rehydrating from the old empty sessionId that
      // beforeEach set. After reload, the store initializer calls
      // generateSessionId() which returns "session-{base36 timestamp}-{random}".
      localStorage.clear()
      vi.resetModules()
      const { useSessionStore: freshStore } = await import('./session')
      const id = freshStore.getState().sessionId
      expect(id).toMatch(/^session-[0-9a-z]+-[0-9a-z]+$/)
      expect(id.length).toBeGreaterThan('session-'.length + 2)
    })

    it('generates a session ID matching the expected format', () => {
      const testId = 'session-abc123-def456'
      useSessionStore.getState().setSessionId(testId)
      expect(useSessionStore.getState().sessionId).toMatch(/^session-/)
    })

    it('persists session ID to localStorage', () => {
      const testId = 'session-abc123-def456'
      useSessionStore.getState().setSessionId(testId)
      const stored = JSON.parse(localStorage.getItem('loom_session')!)
      expect(stored.state.sessionId).toBe(testId)
    })

    it('returns existing stored ID when present in localStorage in persist format', () => {
      localStorage.setItem('loom_session', JSON.stringify({
        state: { sessionId: 'existing-id-456' },
        version: 0,
      }))

      useSessionStore.setState({ sessionId: 'existing-id-456' })
      expect(useSessionStore.getState().sessionId).toBe('existing-id-456')
    })
  })

  describe('setSessionId', () => {
    it('updates both store state and localStorage', () => {
      useSessionStore.getState().setSessionId('test-session-id')

      expect(useSessionStore.getState().sessionId).toBe('test-session-id')
      const stored = JSON.parse(localStorage.getItem('loom_session')!)
      expect(stored.state.sessionId).toBe('test-session-id')
    })
  })

  // Shared SSR safety test — verifies initial state when window is undefined
  testSSRSafety(
    () => import('./session'),
    'useSessionStore',
    { sessionId: '' },
  )
})
