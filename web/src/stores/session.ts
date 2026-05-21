import { create } from 'zustand'

interface SessionStore {
  sessionId: string
  setSessionId: (id: string) => void
}

function getInitialSessionId(): string {
  if (typeof window === 'undefined') return ''
  const stored = localStorage.getItem('loom_session_id')
  if (stored) return stored
  // Generate a default session id if none exists
  const defaultId = `session-${Date.now().toString(36)}`
  localStorage.setItem('loom_session_id', defaultId)
  return defaultId
}

export const useSessionStore = create<SessionStore>((set) => ({
  sessionId: getInitialSessionId(),
  setSessionId: (id: string) => {
    localStorage.setItem('loom_session_id', id)
    set({ sessionId: id })
  },
}))
