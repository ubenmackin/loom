import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface SessionStore {
  sessionId: string
  setSessionId: (id: string) => void
}

function generateSessionId(): string {
  if (typeof window === 'undefined') return ''
  const random = typeof crypto !== 'undefined' && crypto.randomUUID
    ? crypto.randomUUID().slice(0, 8)
    : Math.random().toString(36).slice(2, 10)
  return `session-${Date.now().toString(36)}-${random}`
}

export const useSessionStore = create<SessionStore>()(
  persist(
    (set) => ({
      sessionId: generateSessionId(),
      setSessionId: (id: string) => {
        set({ sessionId: id })
      },
    }),
    {
      name: 'loom_session',
      partialize: (state) => ({ sessionId: state.sessionId }),
    },
  ),
)
