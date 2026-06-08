import { create } from 'zustand'
import { useAuthStore } from './auth'

interface ProjectFilterState {
  selectedProjectId: string | null
  setSelectedProjectId: (id: string | null) => void
  clearProjectFilter: () => void
}

const getInitialProjectId = (): string | null => {
  if (typeof window === 'undefined') return null
  try {
    const user = useAuthStore.getState().user
    if (user) {
      return localStorage.getItem(`loom_selected_project_${user.id}`)
    }
  } catch {
    // Ignore localStorage or hydration errors
  }
  return null
}

export const useProjectFilterStore = create<ProjectFilterState>((set) => ({
  selectedProjectId: getInitialProjectId(),
  setSelectedProjectId: (id: string | null) => {
    set({ selectedProjectId: id })
    try {
      const user = useAuthStore.getState().user
      if (user) {
        if (id) {
          localStorage.setItem(`loom_selected_project_${user.id}`, id)
        } else {
          localStorage.removeItem(`loom_selected_project_${user.id}`)
        }
      }
    } catch {
      // Ignore localStorage errors
    }
  },
  clearProjectFilter: () => {
    set({ selectedProjectId: null })
    try {
      const user = useAuthStore.getState().user
      if (user) {
        localStorage.removeItem(`loom_selected_project_${user.id}`)
      }
    } catch {
      // Ignore localStorage errors
    }
  },
}))

if (typeof window !== 'undefined') {
  let prevUserId = useAuthStore.getState().user?.id
  useAuthStore.subscribe((state) => {
    const userId = state.user?.id
    if (userId !== prevUserId) {
      prevUserId = userId
      if (userId) {
        try {
          const savedProjectId = localStorage.getItem(`loom_selected_project_${userId}`)
          const currentSelected = useProjectFilterStore.getState().selectedProjectId
          if (savedProjectId !== currentSelected) {
            useProjectFilterStore.setState({ selectedProjectId: savedProjectId })
          }
        } catch {
          // Ignore localStorage errors
        }
      } else {
        useProjectFilterStore.setState({ selectedProjectId: null })
      }
    }
  })
}