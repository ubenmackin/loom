import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useSessionStore } from './session'

describe('useSessionStore', () => {
  beforeEach(() => {
    localStorage.clear()
    useSessionStore.setState({ sessionId: '' })
  })

  describe('initial state', () => {
    beforeEach(() => {
      vi.useFakeTimers()
    })

    afterEach(() => {
      vi.useRealTimers()
    })

    it('generates a session ID when none exists in localStorage', async () => {
      vi.setSystemTime(new Date('2025-01-15T12:00:00Z'))

      vi.resetModules()
      const { useSessionStore: freshStore } = await import('./session')

      const now = Date.now()
      const expectedId = `session-${now.toString(36)}`
      expect(freshStore.getState().sessionId).toBe(expectedId)
      expect(localStorage.getItem('loom_session_id')).toBe(expectedId)
    })

    it('returns existing stored ID when present in localStorage', async () => {
      localStorage.setItem('loom_session_id', 'existing-id-456')

      vi.resetModules()
      const { useSessionStore: freshStore } = await import('./session')

      expect(freshStore.getState().sessionId).toBe('existing-id-456')
      expect(localStorage.getItem('loom_session_id')).toBe('existing-id-456')
    })
  })

  describe('setSessionId', () => {
    it('updates both store state and localStorage', () => {
      useSessionStore.getState().setSessionId('test-session-id')

      expect(useSessionStore.getState().sessionId).toBe('test-session-id')
      expect(localStorage.getItem('loom_session_id')).toBe('test-session-id')
    })
  })

  describe('SSR safety', () => {
    afterEach(() => {
      vi.unstubAllGlobals()
    })

    it('returns empty string when window is undefined', async () => {
      vi.stubGlobal('window', undefined)

      vi.resetModules()
      const { useSessionStore: freshStore } = await import('./session')

      expect(freshStore.getState().sessionId).toBe('')
      expect(localStorage.getItem('loom_session_id')).toBeNull()
    })
  })
})
