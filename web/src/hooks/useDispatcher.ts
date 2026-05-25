import { useState, useEffect } from 'react'
import { fetchDispatcherStatus } from '../api/client'
import type { DispatcherStatus, WebSocketEvent } from '../types'

export interface DispatcherEvent {
  type: string
  timestamp: string
  story_id?: string
}

interface UseDispatcherReturn {
  status: DispatcherStatus | null
  dispatcherEvents: DispatcherEvent[]
  isConnected: boolean
}

export function useDispatcher(lastWsEvent?: WebSocketEvent | null): UseDispatcherReturn {
  const [status, setStatus] = useState<DispatcherStatus | null>(null)
  const [dispatcherEvents, setDispatcherEvents] = useState<DispatcherEvent[]>([])
  const [isConnected, setIsConnected] = useState(false)

  // Poll status every 2s
  useEffect(() => {
    let cancelled = false
    const poll = async () => {
      try {
        const data = await fetchDispatcherStatus()
        if (!cancelled) setStatus(data)
      } catch {
        // ignore
      }
    }
    poll()
    const interval = setInterval(poll, 2000)
    return () => {
      cancelled = true
      clearInterval(interval)
    }
  }, [])

  // Process WebSocket events for dispatcher events
  useEffect(() => {
    if (!lastWsEvent) return
    if (lastWsEvent.type === 'dispatcher_event' && lastWsEvent.data) {
      setIsConnected(true)
      const evt = lastWsEvent.data as DispatcherEvent
      setDispatcherEvents(prev => {
        const next = [evt, ...prev]
        return next.slice(0, 200)
      })
    }
  }, [lastWsEvent])

  return { status, dispatcherEvents, isConnected }
}
