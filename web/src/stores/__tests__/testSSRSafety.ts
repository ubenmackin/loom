import { describe, it, expect, afterEach, vi } from 'vitest'

/**
 * Shared SSR safety test for Zustand stores.
 *
 * This helper tests that a store module returns the expected initial state
 * when `window` is undefined (simulating a server-side rendering environment).
 *
 * The `moduleImporter` callback must use `vi.importActual` or a direct dynamic
 * import so that the module is loaded after `window` has been stubbed.
 *
 * @param moduleImporter - Async function that imports and returns the store module
 * @param storeName - The exported store constant name (e.g. 'useSessionStore')
 * @param expectedState - An object of expected state key/value pairs after SSR init
 *
 * Usage:
 * ```ts
 * testSSRSafety(
 *   () => import('./session'),
 *   'useSessionStore',
 *   { sessionId: '' },
 * )
 * ```
 */
export function testSSRSafety(
  moduleImporter: () => Promise<Record<string, unknown>>,
  storeName: string,
  expectedState: Record<string, unknown>,
) {
  describe('SSR safety', () => {
    afterEach(() => {
      vi.unstubAllGlobals()
    })

    it('returns expected initial state when window is undefined', async () => {
      vi.stubGlobal('window', undefined)

      vi.resetModules()
      const mod = await moduleImporter()
      const store = mod[storeName] as { getState: () => Record<string, unknown> }

      const state = store.getState()
      for (const [key, value] of Object.entries(expectedState)) {
        expect(state[key]).toEqual(value)
      }
    })
  })
}