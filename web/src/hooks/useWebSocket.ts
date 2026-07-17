import { useState, useEffect, useRef, createContext, useContext } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import type { WebSocketEvent } from '../types'

const API_URL = import.meta.env.VITE_API_URL || '/api'

// Events that signal data changes and need query invalidation
const RELEVANT_EVENT_TYPES = new Set([
  'board_updated',
  'activity_updated',
  'story_created',
  'story_updated',
  'story_failed',
  'task_created',
  'task_updated',
  'task_deleted',
  'comment_added',
  'session_updated',
  'sessions_updated',
  'gateway_status',
  'dispatcher_status',
  'dispatcher_event',
])

// Map event types to the query keys they should invalidate
const EVENT_QUERY_KEY_MAP: Record<string, string[]> = {
  board_updated: ['board'],
  story_created: ['board'],
  story_updated: ['board'],
  story_failed: ['board'],
  task_created: ['board'],
  task_updated: ['board'],
  task_deleted: ['board'],
  activity_updated: ['activity'],
  session_updated: ['sessions'],
  sessions_updated: ['sessions'],
  gateway_status: ['gateway-status'],
  dispatcher_status: ['dispatcher-status'],
  dispatcher_event: ['dispatcher-status'],
}

function getWsUrl(): string {
  if (API_URL.startsWith('http')) {
    return API_URL.replace(/^http/, 'ws') + '/ws'
  }
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host || 'localhost:8080'
  return `${protocol}//${host}${API_URL}/ws`
}

export interface UseWebSocketReturn {
  isConnected: boolean
  lastEvent: WebSocketEvent | null
}

// ── Context ────────────────────────────────────────────────────────────
// Allows child components (e.g. OperationsPage) to consume WebSocket
// events without calling useWebSocket() directly (avoids duplicate connections).

const WebSocketContext = createContext<UseWebSocketReturn>({
  isConnected: false,
  lastEvent: null,
})

export const useWebSocketEvents = () => useContext(WebSocketContext)

export { WebSocketContext }

// ── Hook ───────────────────────────────────────────────────────────────

export function useWebSocket(): UseWebSocketReturn {
  const [isConnected, setIsConnected] = useState(false)
  const [lastEvent, setLastEvent] = useState<WebSocketEvent | null>(null)
  const queryClient = useQueryClient()
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Debounced invalidation
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({})

  useEffect(() => {
    let cancelled = false
    let retryCount = 0

    const debouncedInvalidate = (key: string) => {
      if (debounceTimers.current[key]) {
        clearTimeout(debounceTimers.current[key])
      }
      debounceTimers.current[key] = setTimeout(() => {
        if (!cancelled) {
          queryClient.invalidateQueries({ queryKey: [key] })
        }
        delete debounceTimers.current[key]
      }, 500)
    }

    const connect = () => {
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
        reconnectTimerRef.current = null
      }

      const url = getWsUrl()
      const ws = new WebSocket(url)
      wsRef.current = ws

      ws.onopen = () => {
        if (cancelled) return
        setIsConnected(true)
        retryCount = 0
      }

      ws.onmessage = (event) => {
        if (cancelled) return
        try {
          const parsed: WebSocketEvent = JSON.parse(event.data)
          setLastEvent(parsed)
          if (RELEVANT_EVENT_TYPES.has(parsed.type)) {
            const keys = EVENT_QUERY_KEY_MAP[parsed.type]
            if (keys) {
              keys.forEach(debouncedInvalidate)
            } else {
              // Fallback: invalidate common keys for recognized but unmapped events
              debouncedInvalidate('board')
              debouncedInvalidate('activity')
              debouncedInvalidate('sessions')
            }
          }
        } catch {
          // ignore malformed messages
        }
      }

      ws.onclose = () => {
        if (cancelled) return
        setIsConnected(false)
        wsRef.current = null

        // Exponential backoff with jitter: 250ms, 500ms, 1s, 2s, 4s… capped at 30s
        const delay = Math.min(250 * Math.pow(2, retryCount) + Math.random() * 1000, 30000)
        retryCount += 1
        reconnectTimerRef.current = setTimeout(connect, delay)
      }

      ws.onerror = () => {
        ws.close()
      }
    }

    connect()

    return () => {
      cancelled = true
      // Clear all debounce timers
      Object.values(debounceTimers.current).forEach(clearTimeout)
      debounceTimers.current = {}
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
      }
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [queryClient])

  return { isConnected, lastEvent }
}
