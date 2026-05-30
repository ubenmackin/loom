import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { useWebSocket } from './useWebSocket'
import type { WebSocketEvent } from '../types'

// ── Fake WebSocket ──────────────────────────────────────────────────────────

class FakeWebSocket {
  static instances: FakeWebSocket[] = []
  url: string
  onopen: (() => void) | null = null
  onclose: ((event: { code?: number; reason?: string }) => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  readyState: number = 0

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }

  close() {
    this.readyState = 3
    this.onclose?.({})
  }

  // Helper to trigger events in tests
  triggerOpen() {
    this.onopen?.()
  }
  triggerMessage(data: string) {
    this.onmessage?.({ data })
  }
  triggerClose() {
    this.onclose?.({})
  }
}

// ── Helpers ─────────────────────────────────────────────────────────────────

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  })
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    )
  }
}

function mockWindowLocation(protocol: string, host: string) {
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { protocol, host },
    writable: true,
  })
}

// ── Tests ───────────────────────────────────────────────────────────────────

describe('useWebSocket', () => {
  beforeEach(() => {
    FakeWebSocket.instances = []
    vi.useFakeTimers()
    globalThis.WebSocket = FakeWebSocket as unknown as typeof WebSocket

    mockWindowLocation('http:', 'localhost:8080')
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('establishes a WebSocket connection on mount with the correct URL', () => {
    renderHook(() => useWebSocket(), { wrapper: createWrapper() })

    expect(FakeWebSocket.instances).toHaveLength(1)
    expect(FakeWebSocket.instances[0].url).toBe('ws://localhost:8080/api/ws')
  })

  it('uses wss:// when the page protocol is https:', () => {
    mockWindowLocation('https:', 'example.com')
    renderHook(() => useWebSocket(), { wrapper: createWrapper() })

    expect(FakeWebSocket.instances[0].url).toBe('wss://example.com/api/ws')
  })

  it('sets isConnected to true when the WebSocket opens', () => {
    const { result } = renderHook(() => useWebSocket(), {
      wrapper: createWrapper(),
    })

    expect(result.current.isConnected).toBe(false)

    const ws = FakeWebSocket.instances[0]
    act(() => {
      ws.triggerOpen()
    })

    expect(result.current.isConnected).toBe(true)
  })

  it('sets lastEvent from a parsed JSON message', () => {
    const { result } = renderHook(() => useWebSocket(), {
      wrapper: createWrapper(),
    })

    const ws = FakeWebSocket.instances[0]
    act(() => {
      ws.triggerOpen()
    })

    const event: WebSocketEvent = { type: 'board_updated', data: { id: 'b-1' } }
    act(() => {
      ws.triggerMessage(JSON.stringify(event))
    })

    expect(result.current.lastEvent).toEqual(event)
  })

  it('invalidates board, activity, and sessions queries on message (debounced)', () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const spy = vi.spyOn(queryClient, 'invalidateQueries')

    renderHook(() => useWebSocket(), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={queryClient}>
          {children}
        </QueryClientProvider>
      ),
    })

    const ws = FakeWebSocket.instances[0]
    act(() => {
      ws.triggerOpen()
      ws.triggerMessage(JSON.stringify({ type: 'board_updated' }))
    })

    // Invalidation is debounced by 500ms, so it hasn't fired yet
    expect(spy).not.toHaveBeenCalled()

    // Advance past the debounce window
    act(() => {
      vi.advanceTimersByTime(500)
    })

    expect(spy).toHaveBeenCalledTimes(3)
    expect(spy).toHaveBeenCalledWith({ queryKey: ['board'] })
    expect(spy).toHaveBeenCalledWith({ queryKey: ['activity'] })
    expect(spy).toHaveBeenCalledWith({ queryKey: ['sessions'] })
  })

  it('does not call invalidateQueries for irrelevant messages (e.g. ping)', () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const spy = vi.spyOn(queryClient, 'invalidateQueries')

    renderHook(() => useWebSocket(), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={queryClient}>
          {children}
        </QueryClientProvider>
      ),
    })

    const ws = FakeWebSocket.instances[0]
    act(() => {
      ws.triggerOpen()
      ws.triggerMessage(JSON.stringify({ type: 'ping' }))
    })

    // Advance past the debounce window to ensure invalidation never fires
    act(() => {
      vi.advanceTimersByTime(500)
    })

    expect(spy).not.toHaveBeenCalled()
  })

  it('does not throw on malformed JSON messages', () => {
    const { result } = renderHook(() => useWebSocket(), {
      wrapper: createWrapper(),
    })

    const ws = FakeWebSocket.instances[0]
    act(() => {
      ws.triggerOpen()
    })

    act(() => {
      expect(() => {
        ws.triggerMessage('not valid json')
      }).not.toThrow()
    })

    // lastEvent should remain null since parsing failed
    expect(result.current.lastEvent).toBeNull()
  })

  it('does not update isConnected or lastEvent after unmount', () => {
    const { result, unmount } = renderHook(() => useWebSocket(), {
      wrapper: createWrapper(),
    })

    const ws = FakeWebSocket.instances[0]

    act(() => {
      unmount()
    })

    // After unmount, triggering events should be no-ops
    act(() => {
      ws.triggerOpen()
    })
    expect(result.current.isConnected).toBe(false)

    act(() => {
      ws.triggerMessage(JSON.stringify({ type: 'test' }))
    })
    expect(result.current.lastEvent).toBeNull()
  })

  it('reconnects with exponential backoff on close (250ms base, capped at 30s)', () => {
    // Mock Math.random to return 0 so jitter doesn't affect exact timing
    vi.spyOn(Math, 'random').mockReturnValue(0)

    renderHook(() => useWebSocket(), { wrapper: createWrapper() })

    // First close → reconnect after 250ms
    let ws = FakeWebSocket.instances[0]
    act(() => {
      ws.triggerOpen()
      ws.triggerClose()
    })

    expect(FakeWebSocket.instances).toHaveLength(1) // only the original so far

    // Advance time by 250ms to trigger first reconnect
    act(() => {
      vi.advanceTimersByTime(250)
    })
    expect(FakeWebSocket.instances).toHaveLength(2)

    // Second close → reconnect after 500ms
    ws = FakeWebSocket.instances[1]
    act(() => {
      ws.triggerOpen()
      ws.triggerClose()
    })

    act(() => {
      vi.advanceTimersByTime(500)
    })
    expect(FakeWebSocket.instances).toHaveLength(3)

    // Third close → reconnect after 1000ms
    ws = FakeWebSocket.instances[2]
    act(() => {
      ws.triggerOpen()
      ws.triggerClose()
    })

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(FakeWebSocket.instances).toHaveLength(4)

    // Fourth close → reconnect after 2000ms
    ws = FakeWebSocket.instances[3]
    act(() => {
      ws.triggerOpen()
      ws.triggerClose()
    })

    act(() => {
      vi.advanceTimersByTime(2000)
    })
    expect(FakeWebSocket.instances).toHaveLength(5)

    // Demonstrate the cap at 30s by advancing many retries
    // Next delays: 4000, 8000, 16000, 30000 (capped), 30000...
    ws = FakeWebSocket.instances[4]
    act(() => {
      ws.triggerOpen()
      ws.triggerClose()
    })

    act(() => {
      vi.advanceTimersByTime(4000)
    })
    expect(FakeWebSocket.instances).toHaveLength(6)

    ws = FakeWebSocket.instances[5]
    act(() => {
      ws.triggerOpen()
      ws.triggerClose()
    })

    act(() => {
      vi.advanceTimersByTime(8000)
    })
    expect(FakeWebSocket.instances).toHaveLength(7)

    ws = FakeWebSocket.instances[6]
    act(() => {
      ws.triggerOpen()
      ws.triggerClose()
    })

    act(() => {
      vi.advanceTimersByTime(16000)
    })
    expect(FakeWebSocket.instances).toHaveLength(8)

    ws = FakeWebSocket.instances[7]
    act(() => {
      ws.triggerOpen()
      ws.triggerClose()
    })

    // Now delay should be capped at 30000
    act(() => {
      vi.advanceTimersByTime(30000)
    })
    expect(FakeWebSocket.instances).toHaveLength(9)

    ws = FakeWebSocket.instances[8]
    act(() => {
      ws.triggerOpen()
      ws.triggerClose()
    })

    // Next reconnect also at 30000 cap
    act(() => {
      vi.advanceTimersByTime(30000)
    })
    expect(FakeWebSocket.instances).toHaveLength(10)
  })

  it('cleans up the WebSocket connection and timers on unmount', () => {
    const { unmount } = renderHook(() => useWebSocket(), {
      wrapper: createWrapper(),
    })

    const ws = FakeWebSocket.instances[0]
    const closeSpy = vi.spyOn(ws, 'close')

    act(() => {
      unmount()
    })

    // close() should have been called
    expect(closeSpy).toHaveBeenCalled()

    // No pending timers should remain (the reconnect timer was cleared)
    // If we advance time, no new WebSocket should appear
    act(() => {
      vi.advanceTimersByTime(100000)
    })
    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  it('sets isConnected to false on close', () => {
    const { result } = renderHook(() => useWebSocket(), {
      wrapper: createWrapper(),
    })

    const ws = FakeWebSocket.instances[0]
    act(() => {
      ws.triggerOpen()
    })
    expect(result.current.isConnected).toBe(true)

    act(() => {
      ws.triggerClose()
    })
    expect(result.current.isConnected).toBe(false)
  })

  it('closes the connection on error', () => {
    renderHook(() => useWebSocket(), { wrapper: createWrapper() })

    const ws = FakeWebSocket.instances[0]
    const closeSpy = vi.spyOn(ws, 'close')

    act(() => {
      ws.onerror?.()
    })

    expect(closeSpy).toHaveBeenCalled()
  })
})
