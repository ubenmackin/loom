import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface ThemeStore {
  isDark: boolean
  toggle: () => void
  setDark: (dark: boolean) => void
}

function applyThemeClass(isDark: boolean) {
  if (typeof window === 'undefined') return
  if (isDark) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

function getInitialDark(): boolean {
  if (typeof document === 'undefined') return true
  return document.documentElement.classList.contains('dark')
}

export const useThemeStore = create<ThemeStore>()(
  persist(
    (set, get) => ({
      isDark: getInitialDark(),
      toggle: () => {
        const next = !get().isDark
        applyThemeClass(next)
        set({ isDark: next })
      },
      setDark: (dark: boolean) => {
        applyThemeClass(dark)
        set({ isDark: dark })
      },
    }),
    {
      name: 'loom_theme',
    },
  ),
)

// Apply theme class on initial load and on every change
applyThemeClass(useThemeStore.getState().isDark)
useThemeStore.subscribe((state) => {
  applyThemeClass(state.isDark)
})
