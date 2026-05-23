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

const applyTheme = (isDark: boolean) => {
  if (typeof window === 'undefined') return
  localStorage.setItem('loom_theme', isDark ? 'dark' : 'light')
  if (isDark) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

export const useThemeStore = create<ThemeStore>((set, get) => ({
  isDark: getInitialDark(),
  toggle: () => {
    const next = !get().isDark
    applyTheme(next)
    set({ isDark: next })
  },
  setDark: (dark: boolean) => {
    applyTheme(dark)
    set({ isDark: dark })
  },
}))
