import { describe, it, expect, beforeEach } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { useThemeStore } from '../stores/theme'
import { useTheme } from './useTheme'

describe('useTheme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    useThemeStore.setState({ isDark: false })
  })

  it('returns the correct isDark value from the store', () => {
    const { result } = renderHook(() => useTheme())
    expect(result.current.isDark).toBe(false)

    act(() => { useThemeStore.setState({ isDark: true }) })
    const { result: result2 } = renderHook(() => useTheme())
    expect(result2.current.isDark).toBe(true)
  })

  it('returns a toggle function that flips the theme', () => {
    const { result } = renderHook(() => useTheme())
    expect(result.current.isDark).toBe(false)

    act(() => { result.current.toggle() })
    expect(result.current.isDark).toBe(true)

    act(() => { result.current.toggle() })
    expect(result.current.isDark).toBe(false)
  })

  it('returns a setDark function that sets the theme', () => {
    const { result } = renderHook(() => useTheme())
    expect(result.current.isDark).toBe(false)

    act(() => { result.current.setDark(true) })
    expect(result.current.isDark).toBe(true)

    act(() => { result.current.setDark(false) })
    expect(result.current.isDark).toBe(false)
  })

  it('toggling via the hook updates the store state', () => {
    const { result } = renderHook(() => useTheme())

    act(() => { result.current.toggle() })
    expect(useThemeStore.getState().isDark).toBe(true)

    act(() => { result.current.toggle() })
    expect(useThemeStore.getState().isDark).toBe(false)
  })
})
