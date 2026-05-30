import { useState, useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import type { WebSocketEvent } from '../types'

const API_URL = import.meta.env.VITE_API_URL || '/api'

// Only invalidate for events that signal data changes
const RELEVANT_EVENT_TYPES = new Set([
  'board_updated',
  'activity_updated',
  'story_created',
  'story_updated',
  'task_created',
  'task_updated',
  'task_deleted',
  'comment_added',
  'session_updated',
  'sessions_updated',
])

function getWsUrl(): string {
  if (API_URL.startsWith('http')) {
    return API_URL.replace(/^http/, 'ws') + '/ws'
  }
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host || 'localhost:8080'
  return `${protocol}//${host}${API_URL}/ws`
}

interface UseWebSocketReturn {
  isConnected: boolean
  lastEvent: WebSocketEvent | null
}

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
            debouncedInvalidate('board')
            debouncedInvalidate('activity')
            debouncedInvalidate('sessions')
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
