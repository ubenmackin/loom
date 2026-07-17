import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useDispatcher } from './useDispatcher'
import type { DispatcherStatus, WebSocketEvent } from '../types'

// ── Fixtures ─────────────────────────────────────────────────────────────────

const sampleStatus: DispatcherStatus = {
  running: true,
  uptime_seconds: 120,
  event_queue_depth: 3,
  events_processed: { task_assigned: 10 },
  started_at: '2025-01-01T00:00:00Z',
  active_sessions: 2,
  ready_tasks: 5,
  pending_build_gates: 1,
  pending_review_gates: 0,
  stale_sessions: 0,
  last_assign_pass: null,
  last_staleness_check: null,
}

// ── Tests ────────────────────────────────────────────────────────────────────

describe('useDispatcher', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('initial state is null/empty/false', () => {
    const { result } = renderHook(() => useDispatcher())

    expect(result.current.status).toBeNull()
    expect(result.current.dispatcherEvents).toEqual([])
    expect(result.current.isConnected).toBe(false)
  })

  it('sets status from dispatcher_status WebSocket event', () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )

    const wsEvent: WebSocketEvent = {
      type: 'dispatcher_status',
      data: sampleStatus,
    }

    rerender(wsEvent)

    expect(result.current.status).toEqual(sampleStatus)
    expect(result.current.isConnected).toBe(true)
  })

  it('updates status on subsequent dispatcher_status events', () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )

    // First status update
    rerender({
      type: 'dispatcher_status',
      data: { ...sampleStatus, uptime_seconds: 120 },
    })

    expect(result.current.status?.uptime_seconds).toBe(120)

    // Second status update
    rerender({
      type: 'dispatcher_status',
      data: { ...sampleStatus, uptime_seconds: 240 },
    })

    expect(result.current.status?.uptime_seconds).toBe(240)
  })

  it('lastWsEvent with type dispatcher_event adds to events and sets isConnected', () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )

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

  it('accumulates multiple dispatcher_event events in order (newest first)', () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )

    rerender({
      type: 'dispatcher_event',
      data: { type: 'event_a', timestamp: '2025-01-01T00:00:01Z' },
    })

    rerender({
      type: 'dispatcher_event',
      data: { type: 'event_b', timestamp: '2025-01-01T00:00:02Z' },
    })

    expect(result.current.dispatcherEvents).toHaveLength(2)
    // Newest first
    expect(result.current.dispatcherEvents[0].type).toBe('event_b')
    expect(result.current.dispatcherEvents[1].type).toBe('event_a')
  })

  it('ignores lastWsEvent when type is not dispatcher_event or dispatcher_status', () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )

    rerender({ type: 'some_other_event', data: { foo: 'bar' } })

    expect(result.current.isConnected).toBe(false)
    expect(result.current.dispatcherEvents).toHaveLength(0)
    expect(result.current.status).toBeNull()
  })

  it('ignores lastWsEvent when data is missing', () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )

    rerender({ type: 'dispatcher_event' })

    expect(result.current.isConnected).toBe(false)
    expect(result.current.dispatcherEvents).toHaveLength(0)
  })

  it('ignores lastWsEvent when it is null', () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )

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

  it('caps events array at 200 items', () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )

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

  it('handles mixed dispatcher_status and dispatcher_event events', () => {
    const { result, rerender } = renderHook(
      (lastWsEvent?: WebSocketEvent | null) => useDispatcher(lastWsEvent),
      { initialProps: undefined },
    )

    // Status event
    rerender({
      type: 'dispatcher_status',
      data: sampleStatus,
    })

    expect(result.current.status).toEqual(sampleStatus)
    expect(result.current.dispatcherEvents).toHaveLength(0)

    // Dispatcher event
    rerender({
      type: 'dispatcher_event',
      data: { type: 'task_assigned', timestamp: '2025-01-01T00:00:00Z' },
    })

    // Status should still be set, events should have 1 entry
    expect(result.current.status).toEqual(sampleStatus)
    expect(result.current.dispatcherEvents).toHaveLength(1)
  })
})
