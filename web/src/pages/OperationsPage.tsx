import { useState, useEffect, useRef } from 'react'
import { useWebSocketEvents } from '../hooks/useWebSocket'
import { useDispatcher } from '../hooks/useDispatcher'
import { fetchGatewayQueue } from '../api/client'
import type { GatewayStatus, GatewayJob, GatewayQueueResponse } from '../api/client'
import Sparkline from '../components/charts/Sparkline'
import DonutChart from '../components/charts/DonutChart'
import HorizontalBarChart from '../components/charts/HorizontalBarChart'
import Gauge from '../components/charts/Gauge'

// ── Chart Color Palettes ───────────────────────────────────────────────

const PROJECT_COLORS = [
  '#6366f1', '#22c55e', '#f59e0b', '#ef4444',
  '#8b5cf6', '#06b6d4', '#ec4899', '#14b8a6',
]

const AGENT_COLORS = [
  '#6366f1', '#22c55e', '#f59e0b', '#ef4444', '#8b5cf6', '#06b6d4',
]

// ── Helpers ────────────────────────────────────────────────────────────

function formatTime(ts: string): string {
  const d = new Date(ts)
  return d.toLocaleTimeString()
}

function formatUptime(seconds: number): string {
  const elapsed = Math.floor(seconds)
  const h = Math.floor(elapsed / 3600)
  const m = Math.floor((elapsed % 3600) / 60)
  const s = elapsed % 60
  return h > 0 ? `${h}h ${m}m ${s}s` : m > 0 ? `${m}m ${s}s` : `${s}s`
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

function eventBorderClass(eventType: string): string {
  switch (eventType) {
    case 'assignment_pass_started':
      return 'border-l-blue-400'
    case 'assignment_pass_finished':
      return 'border-l-green-400'
    case 'gate_check':
      return 'border-l-amber-400'
    case 'staleness_check':
      return 'border-l-red-400'
    default:
      return 'border-l-gray-300 dark:border-l-gray-600'
  }
}

function eventIcon(eventType: string): string {
  switch (eventType) {
    case 'assignment_pass_started':
      return '\u25B6'
    case 'assignment_pass_finished':
      return '\u2713'
    case 'gate_check':
      return '\u25C6'
    case 'staleness_check':
      return '\u26A0'
    default:
      return '\u2022'
  }
}

function eventIconColor(eventType: string): string {
  switch (eventType) {
    case 'assignment_pass_started':
      return 'text-blue-500'
    case 'assignment_pass_finished':
      return 'text-green-500'
    case 'gate_check':
      return 'text-amber-500'
    case 'staleness_check':
      return 'text-red-500'
    default:
      return 'text-gray-400'
  }
}

// ── Component ──────────────────────────────────────────────────────────

export default function OperationsPage() {
  const [activeTab, setActiveTab] = useState<'events' | 'sessions'>('events')

  // ── WebSocket Events ──
  const { lastEvent, isConnected } = useWebSocketEvents()
  const { status: dispatcherStatus, dispatcherEvents } = useDispatcher(lastEvent)

  // ── Gateway State (from WebSocket) ──
  const [gatewayStatus, setGatewayStatus] = useState<GatewayStatus | null>(null)
  const [queue, setQueue] = useState<GatewayQueueResponse | null>(null)

  // ── Uptime ──
  const [dispatcherUptime, setDispatcherUptime] = useState('0s')
  const [gatewayUptime, setGatewayUptime] = useState('0s')

  useEffect(() => {
    const update = () => {
      if (dispatcherStatus?.uptime_seconds != null) {
        setDispatcherUptime(formatUptime(dispatcherStatus.uptime_seconds))
      }
    }
    update()
    const interval = setInterval(update, 1000)
    return () => clearInterval(interval)
  }, [dispatcherStatus?.uptime_seconds])

  useEffect(() => {
    const update = () => {
      if (gatewayStatus?.uptime_seconds != null) {
        setGatewayUptime(formatUptime(gatewayStatus.uptime_seconds))
      }
    }
    update()
    const interval = setInterval(update, 1000)
    return () => clearInterval(interval)
  }, [gatewayStatus?.uptime_seconds])

  // ── Gateway status from WebSocket ──
  useEffect(() => {
    if (!lastEvent || lastEvent.type !== 'gateway_status') return
    setGatewayStatus(lastEvent.data as GatewayStatus)
  }, [lastEvent])

  // ── Queue fetch on gateway status update ──
  useEffect(() => {
    if (!lastEvent || lastEvent.type !== 'gateway_status') return
    let cancelled = false
    fetchGatewayQueue()
      .then((q) => { if (!cancelled) setQueue(q) })
      .catch(() => { /* ignore */ })
    return () => { cancelled = true }
  }, [lastEvent])

  // Initial queue fetch on mount
  useEffect(() => {
    let cancelled = false
    fetchGatewayQueue()
      .then((q) => { if (!cancelled) setQueue(q) })
      .catch(() => { /* ignore */ })
    return () => { cancelled = true }
  }, [])

  // ── Sparkline: Active Sessions History ──
  const sessionHistoryRef = useRef<number[]>([])
  const [sessionHistory, setSessionHistory] = useState<number[]>([])

  useEffect(() => {
    if (gatewayStatus?.active_sessions != null) {
      sessionHistoryRef.current = [
        ...sessionHistoryRef.current.slice(-29),
        gatewayStatus.active_sessions,
      ]
      setSessionHistory([...sessionHistoryRef.current])
    }
  }, [gatewayStatus?.active_sessions])

  // ── Sparkline: Events Throughput (per minute) ──
  const throughputRef = useRef<{ total: number; time: number }[]>([])
  const [throughputData, setThroughputData] = useState<number[]>([])

  useEffect(() => {
    if (dispatcherStatus?.events_processed) {
      const total = Object.values(dispatcherStatus.events_processed).reduce(
        (a, b) => a + b,
        0,
      )
      const now = Date.now()
      const ref = throughputRef.current
      const prev = ref[ref.length - 1]
      let rate = 0
      if (prev) {
        const elapsed = (now - prev.time) / 1000
        if (elapsed > 0) {
          rate = Math.round(((total - prev.total) / elapsed) * 60)
        }
      }
      throughputRef.current = [...ref.slice(-29), { total, time: now }]
      setThroughputData((prev) => [...prev.slice(-29), Math.max(0, rate)])
    }
  }, [dispatcherStatus])

  // ── Derived Data ──
  const sessionsByProject = gatewayStatus?.sessions_by_project ?? []
  const sessionsByAgentEntries = gatewayStatus?.sessions_by_agent
    ? Object.entries(gatewayStatus.sessions_by_agent)
    : []

  const donutData = sessionsByProject.map((p, i) => ({
    name: p.project_name,
    value: p.count,
    color: PROJECT_COLORS[i % PROJECT_COLORS.length],
  }))

  const barData = sessionsByAgentEntries.map(([name, value], i) => ({
    name,
    value,
    color: AGENT_COLORS[i % AGENT_COLORS.length],
  }))

  const queueDepth = gatewayStatus?.queue_depth ?? dispatcherStatus?.event_queue_depth ?? 0
  const gaugeMax = Math.max(queueDepth * 2, 10)

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {/* ── System Health Header ──────────────────────────────────── */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-0">
          {/* Dispatcher */}
          <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
            <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">
              Dispatcher
            </div>
            <div className="flex items-center gap-2">
              <span
                className={`inline-block w-3 h-3 rounded-full ${
                  dispatcherStatus?.running ? 'bg-green-500' : 'bg-red-500'
                }`}
              />
              <span className="font-mono font-semibold dark:text-neutral-200">
                {dispatcherStatus?.running ? 'Running' : 'Stopped'}
              </span>
              <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted ml-auto">
                {dispatcherUptime}
              </span>
            </div>
          </div>
          {/* Gateway */}
          <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
            <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">
              Gateway
            </div>
            <div className="flex items-center gap-2">
              <span
                className={`inline-block w-3 h-3 rounded-full ${
                  gatewayStatus?.running ? 'bg-green-500' : 'bg-red-500'
                }`}
              />
              <span className="font-mono font-semibold dark:text-neutral-200">
                {gatewayStatus?.running ? 'Running' : 'Stopped'}
              </span>
              <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted ml-auto">
                {gatewayUptime}
              </span>
            </div>
          </div>
          {/* WebSocket */}
          <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
            <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">
              WebSocket
            </div>
            <div className="flex items-center gap-2">
              <span
                className={`inline-block w-3 h-3 rounded-full ${
                  isConnected ? 'bg-green-500' : 'bg-red-500'
                }`}
              />
              <span className="font-mono font-semibold dark:text-neutral-200">
                {isConnected ? 'Connected' : 'Disconnected'}
              </span>
            </div>
          </div>
        </div>

        {/* ── Tab Bar ──────────────────────────────────────────────── */}
        <div className="flex gap-0 mb-4">
          <button
            onClick={() => setActiveTab('events')}
            className={`px-4 py-2 text-xs font-mono uppercase tracking-widest ${
              activeTab === 'events'
                ? 'bg-white dark:bg-charcoal-dark border-b-2 border-purple-active text-neutral-900 dark:text-neutral-200'
                : 'bg-gray-50 dark:bg-charcoal-darkest text-neutral-500 dark:text-amber-muted hover:text-neutral-700'
            }`}
          >
            Events
          </button>
          <button
            onClick={() => setActiveTab('sessions')}
            className={`px-4 py-2 text-xs font-mono uppercase tracking-widest ${
              activeTab === 'sessions'
                ? 'bg-white dark:bg-charcoal-dark border-b-2 border-purple-active text-neutral-900 dark:text-neutral-200'
                : 'bg-gray-50 dark:bg-charcoal-darkest text-neutral-500 dark:text-amber-muted hover:text-neutral-700'
            }`}
          >
            Sessions
          </button>
        </div>

        {activeTab === 'events' ? (
          <>
            {/* ── Events Throughput Sparkline ────────────────────────── */}
            <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
              <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-2">
                Events Per Minute
              </div>
              {throughputData.length > 1 ? (
                <Sparkline data={throughputData} color="#6366f1" height={60} />
              ) : (
                <div className="h-[60px] flex items-center justify-center">
                  <span className="text-xs font-mono text-neutral-400 dark:text-amber-muted italic">
                    Collecting data...
                  </span>
                </div>
              )}
            </div>

            {/* ── Live Event Feed ────────────────────────────────────── */}
            <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
              <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">
                Live Event Feed{' '}
                {dispatcherEvents.length > 0 && (
                  <span className="text-xs">({dispatcherEvents.length})</span>
                )}
              </div>
              <div className="h-64 overflow-y-auto space-y-0 font-mono text-xs">
                {dispatcherEvents.length === 0 ? (
                  <p className="text-neutral-400 dark:text-amber-muted italic">
                    Waiting for dispatcher events...
                  </p>
                ) : (
                  [...dispatcherEvents].reverse().map((evt, i) => (
                    <div
                      key={i}
                      className={`flex items-center gap-2 py-1.5 border-b border-gray-200 dark:border-gray-border last:border-0 border-l-4 pl-2 ${eventBorderClass(evt.type)}`}
                    >
                      <span className={`text-xs w-4 text-center shrink-0 ${eventIconColor(evt.type)}`}>
                        {eventIcon(evt.type)}
                      </span>
                      <span className="text-neutral-400 w-16 shrink-0">{formatTime(evt.timestamp)}</span>
                      <span
                        className={`text-[10px] px-1.5 py-0.5 font-mono ${eventBadgeColor(evt.type)}`}
                      >
                        {evt.type}
                      </span>
                      {evt.story_id && (
                        <span className="text-neutral-500 truncate">
                          story: {evt.story_id.substring(0, 8)}
                        </span>
                      )}
                    </div>
                  ))
                )}
              </div>
            </div>

            {/* ── Pipeline Panels ────────────────────────────────────── */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-0">
              {/* Assignment Pipeline */}
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">
                  Assignment Pipeline
                </div>
                <div className="space-y-2">
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">
                      Ready Tasks
                    </span>
                    <span className="font-mono font-bold dark:text-neutral-200">
                      {dispatcherStatus?.ready_tasks ?? 0}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">
                      Active Sessions
                    </span>
                    <span className="font-mono font-bold dark:text-neutral-200">
                      {dispatcherStatus?.active_sessions ?? 0}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">
                      Last Pass
                    </span>
                    <span className="font-mono text-neutral-500 dark:text-neutral-400">
                      {dispatcherStatus?.last_assign_pass
                        ? formatTime(dispatcherStatus.last_assign_pass)
                        : '—'}
                    </span>
                  </div>
                </div>
              </div>

              {/* Gate Pipeline */}
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">
                  Gate Pipeline
                </div>
                <div className="space-y-2">
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">
                      Pending Build Gates
                    </span>
                    <span className="font-mono font-bold dark:text-neutral-200">
                      {dispatcherStatus?.pending_build_gates ?? 0}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">
                      Pending Review Gates
                    </span>
                    <span className="font-mono font-bold dark:text-neutral-200">
                      {dispatcherStatus?.pending_review_gates ?? 0}
                    </span>
                  </div>
                </div>
              </div>

              {/* Staleness Monitor */}
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">
                  Staleness Monitor
                </div>
                <div className="space-y-2">
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">
                      Stale Sessions
                    </span>
                    <span className="font-mono font-bold dark:text-neutral-200">
                      {dispatcherStatus?.stale_sessions ?? 0}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">
                      Last Check
                    </span>
                    <span className="font-mono text-neutral-500 dark:text-neutral-400">
                      {dispatcherStatus?.last_staleness_check
                        ? formatTime(dispatcherStatus.last_staleness_check)
                        : '—'}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </>
        ) : (
          <>
            {/* ── Active Sessions Sparkline ──────────────────────────── */}
            <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
              <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-2">
                Active Sessions
              </div>
              {sessionHistory.length > 1 ? (
                <Sparkline data={sessionHistory} color="#22c55e" height={60} />
              ) : (
                <div className="h-[60px] flex items-center justify-center">
                  <span className="font-mono text-lg font-semibold dark:text-neutral-200">
                    {gatewayStatus?.active_sessions ?? '—'}
                  </span>
                </div>
              )}
            </div>

            {/* ── Charts Row: Donut + Bar ────────────────────────────── */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-0">
              {/* Sessions by Project (Donut) */}
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">
                  Sessions by Project
                </div>
                {donutData.length === 0 ? (
                  <p className="font-mono text-xs text-neutral-400 dark:text-amber-muted italic">
                    No session data available.
                  </p>
                ) : (
                  <div className="flex justify-center">
                    <DonutChart data={donutData} size={160} showLegend />
                  </div>
                )}
              </div>

              {/* Sessions by Agent (Horizontal Bar) */}
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">
                  Sessions by Agent
                </div>
                {barData.length === 0 ? (
                  <p className="font-mono text-xs text-neutral-400 dark:text-amber-muted italic">
                    No session data available.
                  </p>
                ) : (
                  <HorizontalBarChart data={barData} />
                )}
              </div>
            </div>

            {/* ── Queue Depth Gauge + Queue Table ───────────────────── */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-0">
              {/* Queue Depth Gauge */}
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3 flex flex-col items-center justify-center">
                <Gauge value={queueDepth} max={gaugeMax} label="Queue Depth" />
              </div>

              {/* Queue Table */}
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">
                  Queue {queue != null && <span className="text-xs">({queue.total})</span>}
                </div>
                {queue == null || queue.jobs.length === 0 ? (
                  <p className="font-mono text-xs text-neutral-400 dark:text-amber-muted italic">
                    Queue is empty.
                  </p>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full font-mono text-xs">
                      <thead>
                        <tr className="border-b border-gray-200 dark:border-gray-border">
                          <th className="text-left py-2 pr-4 text-neutral-500 dark:text-amber-muted uppercase tracking-widest">
                            Project
                          </th>
                          <th className="text-left py-2 pr-4 text-neutral-500 dark:text-amber-muted uppercase tracking-widest">
                            Agent
                          </th>
                          <th className="text-left py-2 pr-4 text-neutral-500 dark:text-amber-muted uppercase tracking-widest">
                            Task
                          </th>
                          <th className="text-left py-2 text-neutral-500 dark:text-amber-muted uppercase tracking-widest">
                            Created
                          </th>
                        </tr>
                      </thead>
                      <tbody>
                        {queue.jobs.map((job: GatewayJob) => (
                          <tr
                            key={job.id}
                            className="border-b border-gray-200 dark:border-gray-border last:border-0"
                          >
                            <td className="py-2 pr-4 text-neutral-700 dark:text-neutral-300">
                              {job.project_name || job.project_id}
                            </td>
                            <td className="py-2 pr-4 text-neutral-700 dark:text-neutral-300">
                              {job.agent_type}
                            </td>
                            <td className="py-2 pr-4 text-neutral-700 dark:text-neutral-300">
                              <span className="font-mono">{job.task_id.substring(0, 8)}</span>
                            </td>
                            <td className="py-2 text-neutral-500 dark:text-amber-muted">
                              {formatTime(job.created_at)}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
