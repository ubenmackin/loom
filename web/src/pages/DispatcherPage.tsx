import { useState, useEffect } from 'react'
import { useWebSocket } from '../hooks/useWebSocket'
import { useDispatcher, type DispatcherEvent } from '../hooks/useDispatcher'
import SharpTag from '../components/SharpTag'
import type { DispatcherStatus } from '../types'

function formatTime(ts: string): string {
  const d = new Date(ts)
  return d.toLocaleTimeString()
}

function eventBadgeColor(eventType: string): string {
  switch (eventType) {
    case 'assignment_pass_started':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300'
    case 'assignment_pass_finished':
      return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300'
    case 'gate_check':
      return 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300'
    case 'staleness_check':
      return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300'
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300'
  }
}

export default function DispatcherPage() {
  const ws = useWebSocket()
  const { status, dispatcherEvents, isConnected } = useDispatcher(ws.lastEvent)
  const [uptime, setUptime] = useState('0s')

  // Update uptime every second
  useEffect(() => {
    const update = () => {
      if (status?.uptime_seconds != null) {
        const elapsed = Math.floor(status.uptime_seconds)
        const h = Math.floor(elapsed / 3600)
        const m = Math.floor((elapsed % 3600) / 60)
        const s = elapsed % 60
        setUptime(h > 0 ? `${h}h ${m}m ${s}s` : m > 0 ? `${m}m ${s}s` : `${s}s`)
      }
    }
    update()
    const interval = setInterval(update, 1000)
    return () => clearInterval(interval)
  }, [status?.uptime_seconds])

  return (
    <div className="p-3 space-y-2">
      <h1 className="text-2xl font-bold text-gray-900 dark:text-white font-mono">Dispatcher Dashboard</h1>

      {/* Status Bar */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-0">
        <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-2">
          <div className="text-sm text-gray-500 dark:text-amber-muted font-mono mb-1">Status</div>
          <div className="flex items-center gap-2">
            <span className={`inline-block w-3 h-3 rounded-full ${status?.running ? 'bg-green-500' : 'bg-red-500'}`} />
            <span className="font-mono font-semibold dark:text-neutral-200">{status?.running ? 'Running' : 'Stopped'}</span>
          </div>
        </div>
        <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-2">
          <div className="text-sm text-gray-500 dark:text-amber-muted font-mono mb-1">Uptime</div>
          <div className="font-mono text-lg font-semibold dark:text-neutral-200">{uptime}</div>
        </div>
        <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-2">
          <div className="text-sm text-gray-500 dark:text-amber-muted font-mono mb-1">Queue Depth</div>
          <div className="font-mono text-lg font-semibold dark:text-neutral-200">{status?.event_queue_depth ?? '—'}</div>
        </div>
        <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-2">
          <div className="text-sm text-gray-500 dark:text-amber-muted font-mono mb-1">WebSocket</div>
          <div className="flex items-center gap-2">
            <span className={`inline-block w-3 h-3 rounded-full ${isConnected ? 'bg-green-500' : 'bg-red-500'}`} />
            <span className="font-mono dark:text-neutral-200">{isConnected ? 'Connected' : 'Disconnected'}</span>
          </div>
        </div>
      </div>

      {/* Events Processed */}
      {status?.events_processed && (
        <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-2">
          <h2 className="text-sm text-gray-500 dark:text-amber-muted font-mono mb-3 uppercase tracking-wider">Events Processed</h2>
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-0">
            {Object.entries(status.events_processed).map(([key, value]) => (
              <div key={key} className="text-center">
                <div className="font-mono text-xl font-bold dark:text-neutral-200">{value}</div>
                <div className="text-xs text-gray-500 dark:text-amber-muted font-mono truncate">{key}</div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Event Feed */}
      <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-2">
        <h2 className="text-sm text-gray-500 dark:text-amber-muted font-mono mb-3 uppercase tracking-wider">
          Live Event Feed {dispatcherEvents.length > 0 && <span className="text-xs">({dispatcherEvents.length})</span>}
        </h2>
        <div className="h-64 overflow-y-auto space-y-1 font-mono text-xs">
          {dispatcherEvents.length === 0 ? (
            <p className="text-gray-400 dark:text-amber-muted italic">Waiting for dispatcher events...</p>
          ) : (
            [...dispatcherEvents].reverse().map((evt, i) => (
              <div key={i} className="flex items-center gap-2 py-1 border-b border-gray-100 dark:border-gray-800 last:border-0">
                <span className="text-gray-400 w-16 shrink-0">{formatTime(evt.timestamp)}</span>
                <span className={`text-[10px] px-1.5 py-0.5 font-mono ${eventBadgeColor(evt.type)}`}>
                  {evt.type}
                </span>
                {evt.story_id && <span className="text-gray-500 truncate">story: {evt.story_id.substring(0, 8)}</span>}
              </div>
            ))
          )}
        </div>
      </div>

      {/* Pipeline Panels */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-0">
        {/* Assignment Pipeline */}
        <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-2">
          <h2 className="text-sm text-gray-500 dark:text-amber-muted font-mono mb-3 uppercase tracking-wider">Assignment Pipeline</h2>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="font-mono text-sm text-gray-500 dark:text-amber-muted">Ready Tasks</span>
              <span className="font-mono font-bold dark:text-neutral-200">—</span>
            </div>
            <div className="flex justify-between">
              <span className="font-mono text-sm text-gray-500 dark:text-amber-muted">Active Sessions</span>
              <span className="font-mono font-bold dark:text-neutral-200">—</span>
            </div>
            <div className="flex justify-between">
              <span className="font-mono text-sm text-gray-500 dark:text-amber-muted">Last Pass</span>
              <span className="font-mono text-gray-500 dark:text-neutral-400">—</span>
            </div>
          </div>
        </div>

        {/* Gate Pipeline */}
        <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-2">
          <h2 className="text-sm text-gray-500 dark:text-amber-muted font-mono mb-3 uppercase tracking-wider">Gate Pipeline</h2>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="font-mono text-sm text-gray-500 dark:text-amber-muted">Pending Build Gates</span>
              <span className="font-mono font-bold dark:text-neutral-200">—</span>
            </div>
            <div className="flex justify-between">
              <span className="font-mono text-sm text-gray-500 dark:text-amber-muted">Pending Review Gates</span>
              <span className="font-mono font-bold dark:text-neutral-200">—</span>
            </div>
          </div>
        </div>

        {/* Staleness Monitor */}
        <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-2">
          <h2 className="text-sm text-gray-500 dark:text-amber-muted font-mono mb-3 uppercase tracking-wider">Staleness Monitor</h2>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="font-mono text-sm text-gray-500 dark:text-amber-muted">Stale Sessions</span>
              <span className="font-mono font-bold dark:text-neutral-200">—</span>
            </div>
            <div className="flex justify-between">
              <span className="font-mono text-sm text-gray-500 dark:text-amber-muted">Last Check</span>
              <span className="font-mono text-gray-500 dark:text-neutral-400">—</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
