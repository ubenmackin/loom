import { useThemeStore } from './theme'

describe('useThemeStore', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    useThemeStore.setState({ isDark: false })
  })

  describe('initial dark mode detection', () => {
    it('reads isDark=true when localStorage has "dark"', async () => {
      localStorage.setItem('loom_theme', 'dark')
      vi.resetModules()
      const { useThemeStore: store } = await import('./theme')

      expect(store.getState().isDark).toBe(true)
    })

    it('reads isDark=false when localStorage has "light" even if class is "dark"', async () => {
      localStorage.setItem('loom_theme', 'light')
      document.documentElement.classList.add('dark')
      vi.resetModules()
      const { useThemeStore: store } = await import('./theme')

      expect(store.getState().isDark).toBe(false)
    })

    it('falls back to document class when localStorage is absent and class is "dark"', async () => {
      document.documentElement.classList.add('dark')
      vi.resetModules()
      const { useThemeStore: store } = await import('./theme')

      expect(store.getState().isDark).toBe(true)
    })

    it('falls back to false when localStorage is absent and class is not "dark"', async () => {
      vi.resetModules()
      const { useThemeStore: store } = await import('./theme')

      expect(store.getState().isDark).toBe(false)
    })
  })

  describe('toggle', () => {
    it('flips isDark from false to true and updates localStorage + class', () => {
      useThemeStore.setState({ isDark: false })
      expect(useThemeStore.getState().isDark).toBe(false)

      useThemeStore.getState().toggle()

      expect(useThemeStore.getState().isDark).toBe(true)
      expect(document.documentElement.classList.contains('dark')).toBe(true)
      expect(localStorage.getItem('loom_theme')).toBe('dark')
    })

    it('flips isDark from true to false and updates localStorage + class', () => {
      document.documentElement.classList.add('dark')
      localStorage.setItem('loom_theme', 'dark')
      useThemeStore.setState({ isDark: true })
      expect(useThemeStore.getState().isDark).toBe(true)

      useThemeStore.getState().toggle()

      expect(useThemeStore.getState().isDark).toBe(false)
      expect(document.documentElement.classList.contains('dark')).toBe(false)
      expect(localStorage.getItem('loom_theme')).toBe('light')
    })
  })

  describe('setDark', () => {
    it('setDark(true) applies dark theme', () => {
      useThemeStore.getState().setDark(true)

      expect(useThemeStore.getState().isDark).toBe(true)
      expect(document.documentElement.classList.contains('dark')).toBe(true)
      expect(localStorage.getItem('loom_theme')).toBe('dark')
    })

    it('setDark(false) applies light theme', () => {
      // Start with dark
      document.documentElement.classList.add('dark')
      localStorage.setItem('loom_theme', 'dark')
      useThemeStore.setState({ isDark: true })

      useThemeStore.getState().setDark(false)

      expect(useThemeStore.getState().isDark).toBe(false)
      expect(document.documentElement.classList.contains('dark')).toBe(false)
      expect(localStorage.getItem('loom_theme')).toBe('light')
    })
  })

  describe('SSR safety', () => {
    afterEach(() => {
      vi.unstubAllGlobals()
    })

    it('getInitialDark() returns true when window is undefined', async () => {
      vi.stubGlobal('window', undefined)
      vi.resetModules()

      const { useThemeStore: ssrStore } = await import('./theme')

      expect(ssrStore.getState().isDark).toBe(true)
    })

    it('toggle() does not throw when window is undefined', async () => {
      vi.stubGlobal('window', undefined)
      vi.resetModules()

      const { useThemeStore: ssrStore } = await import('./theme')

      expect(() => ssrStore.getState().toggle()).not.toThrow()
    })

    it('setDark() does not throw when window is undefined', async () => {
      vi.stubGlobal('window', undefined)
      vi.resetModules()

      const { useThemeStore: ssrStore } = await import('./theme')

      expect(() => ssrStore.getState().setDark(true)).not.toThrow()
      expect(() => ssrStore.getState().setDark(false)).not.toThrow()
    })
  })
})
