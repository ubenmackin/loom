import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useDispatcher } from './useDispatcher'
import type { DispatcherStatus, WebSocketEvent } from '../types'

// Mock the API client before any imports that use it
vi.mock('../api/client', () => ({
  fetchDispatcherStatus: vi.fn(),
}))

import { fetchDispatcherStatus } from '../api/client'

const mockFetchDispatcherStatus = vi.mocked(fetchDispatcherStatus)

// ── Fixtures ─────────────────────────────────────────────────────────────────

const sampleStatus: DispatcherStatus = {
  running: true,
  uptime_seconds: 120,
  event_queue_depth: 3,
  events_processed: { task_assigned: 10 },
  started_at: '2025-01-01T00:00:00Z',
}

// ── Tests ────────────────────────────────────────────────────────────────────

describe('useDispatcher', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('initial status is null and isConnected is false', async () => {
    const { result } = renderHook(() => useDispatcher())

    // Check state before the async poll's microtask resolves
    // (useEffect fires synchronously, but the await inside poll()
    //  creates a pending microtask — the initial useState(null) is still visible)
    expect(result.current.status).toBeNull()
    expect(result.current.dispatcherEvents).toEqual([])
    expect(result.current.isConnected).toBe(false)

    // Flush pending microtask so the state update is wrapped in act()
    await act(async () => {})
  })

  it('polls fetchDispatcherStatus every 2 seconds', async () => {
    mockFetchDispatcherStatus.mockResolvedValue(sampleStatus)

    renderHook(() => useDispatcher())
    await act(async () => {}) // flush initial poll

    // Initial call on mount
    expect(mockFetchDispatcherStatus).toHaveBeenCalledTimes(1)

    // Advance 2s → second poll
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(mockFetchDispatcherStatus).toHaveBeenCalledTimes(2)

    // Advance another 2s → third poll
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(mockFetchDispatcherStatus).toHaveBeenCalledTimes(3)
  })

  it('sets status from polling response', async () => {
    mockFetchDispatcherStatus.mockResolvedValue(sampleStatus)

    const { result } = renderHook(() => useDispatcher())
    // Flush the initial poll's async promise + React state update
    await act(async () => {})

    expect(result.current.status).toEqual(sampleStatus)
  })

  it('continues polling even when fetchDispatcherStatus fails', async () => {
    mockFetchDispatcherStatus.mockRejectedValue(new Error('Network error'))

    renderHook(() => useDispatcher())
    // Flush the initial poll (which will fail, but should not crash)
    await act(async () => {})

    // The hook caught the error (ignored), so the interval should still be active.
    // Advance 2s — second poll should fire even though the first one failed.
    mockFetchDispatcherStatus.mockResolvedValue(sampleStatus)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })

    // fetchDispatcherStatus should have been called twice:
    // once on mount (rejected) and once after 2s interval (resolved)
    expect(mockFetchDispatcherStatus).toHaveBeenCalledTimes(2)
  })

  it('lastWsEvent with type dispatcher_event adds to events and sets isConnected', async () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )
    await act(async () => {}) // flush initial poll

    const wsEvent: WebSocketEvent = {
      type: 'dispatcher_event',
      data: {
        type: 'task_assigned',
        timestamp: '2025-01-01T00:00:00Z',
        story_id: 'story-1',
      },
    }

    rerender(wsEvent)

    expect(result.current.isConnected).toBe(true)
    expect(result.current.dispatcherEvents).toHaveLength(1)
    expect(result.current.dispatcherEvents[0]).toEqual({
      type: 'task_assigned',
      timestamp: '2025-01-01T00:00:00Z',
      story_id: 'story-1',
    })
  })

  it('ignores lastWsEvent when type is not dispatcher_event', async () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )
    await act(async () => {}) // flush initial poll

    rerender({ type: 'some_other_event', data: { foo: 'bar' } })

    expect(result.current.isConnected).toBe(false)
    expect(result.current.dispatcherEvents).toHaveLength(0)
  })

  it('ignores lastWsEvent when data is missing', async () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )
    await act(async () => {}) // flush initial poll

    rerender({ type: 'dispatcher_event' })

    expect(result.current.isConnected).toBe(false)
    expect(result.current.dispatcherEvents).toHaveLength(0)
  })

  it('ignores lastWsEvent when it is null', async () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )
    await act(async () => {}) // flush initial poll

    // Start with an event to verify state is set
    rerender({
      type: 'dispatcher_event',
      data: { type: 'task_assigned', timestamp: '2025-01-01T00:00:00Z' },
    })

    expect(result.current.dispatcherEvents).toHaveLength(1)

    // Now pass null — should not clear existing events, but also not add
    rerender(null)

    expect(result.current.dispatcherEvents).toHaveLength(1)
  })

  it('caps events array at 200 items', async () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )
    await act(async () => {}) // flush initial poll

    // Add 250 events
    for (let i = 0; i < 250; i++) {
      rerender({
        type: 'dispatcher_event',
        data: {
          type: `event_${i}`,
          timestamp: `2025-01-01T00:00:${String(i).padStart(2, '0')}Z`,
        },
      })
    }

    expect(result.current.dispatcherEvents).toHaveLength(200)

    // Most recent event is at index 0 (prepended)
    expect(result.current.dispatcherEvents[0].type).toBe('event_249')
    // Oldest kept event should be event_50
    expect(result.current.dispatcherEvents[199].type).toBe('event_50')
  })

  it('cleans up polling interval on unmount', async () => {
    mockFetchDispatcherStatus.mockResolvedValue(sampleStatus)

    const { unmount } = renderHook(() => useDispatcher())
    await act(async () => {}) // flush initial poll

    // Clear the mock call count from the initial poll
    vi.clearAllMocks()

    // Unmount — this should clear the interval
    unmount()

    // Advance time significantly — no new polls should fire
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000)
    })
    expect(mockFetchDispatcherStatus).not.toHaveBeenCalled()
  })
})
