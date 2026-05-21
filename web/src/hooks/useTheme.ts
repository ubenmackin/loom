import { useThemeStore } from '../stores/theme'

export function useTheme() {
  const isDark = useThemeStore((s) => s.isDark)
  const toggle = useThemeStore((s) => s.toggle)
  const setDark = useThemeStore((s) => s.setDark)

  return { isDark, toggle, setDark }
}
