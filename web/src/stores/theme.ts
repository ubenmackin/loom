import { create } from 'zustand'

interface ThemeStore {
  isDark: boolean
  toggle: () => void
  setDark: (dark: boolean) => void
}

function getInitialDark(): boolean {
  if (typeof window === 'undefined') return true
  const stored = localStorage.getItem('loom_theme')
  if (stored === 'dark') return true
  if (stored === 'light') return false
  return document.documentElement.classList.contains('dark')
}

export const useThemeStore = create<ThemeStore>((set) => ({
  isDark: getInitialDark(),
  toggle: () =>
    set((state) => {
      const next = !state.isDark
      localStorage.setItem('loom_theme', next ? 'dark' : 'light')
      if (next) {
        document.documentElement.classList.add('dark')
      } else {
        document.documentElement.classList.remove('dark')
      }
      return { isDark: next }
    }),
  setDark: (dark: boolean) =>
    set(() => {
      localStorage.setItem('loom_theme', dark ? 'dark' : 'light')
      if (dark) {
        document.documentElement.classList.add('dark')
      } else {
        document.documentElement.classList.remove('dark')
      }
      return { isDark: dark }
    }),
}))
