import { useState, useEffect } from 'react'
import { useWebSocket } from '../hooks/useWebSocket'
import { useDispatcher } from '../hooks/useDispatcher'
import { fetchGatewayStatus, fetchGatewayQueue } from '../api/client'
import type { GatewayStatus, GatewayJob, GatewayQueueResponse } from '../api/client'

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

export default function OperationsPage() {
  const [activeTab, setActiveTab] = useState<'events' | 'sessions'>('events')

  // ── Events Tab State ──
  const ws = useWebSocket()
  const { status: dispatcherStatus, dispatcherEvents, isConnected } = useDispatcher(ws.lastEvent)
  const [dispatcherUptime, setDispatcherUptime] = useState('0s')

  // Update dispatcher uptime every second
  useEffect(() => {
    const update = () => {
      if (dispatcherStatus?.uptime_seconds != null) {
        const elapsed = Math.floor(dispatcherStatus.uptime_seconds)
        const h = Math.floor(elapsed / 3600)
        const m = Math.floor((elapsed % 3600) / 60)
        const s = elapsed % 60
        setDispatcherUptime(h > 0 ? `${h}h ${m}m ${s}s` : m > 0 ? `${m}m ${s}s` : `${s}s`)
      }
    }
    update()
    const interval = setInterval(update, 1000)
    return () => clearInterval(interval)
  }, [dispatcherStatus?.uptime_seconds])

  // ── Sessions Tab State ──
  const [gatewayStatus, setGatewayStatus] = useState<GatewayStatus | null>(null)
  const [queue, setQueue] = useState<GatewayQueueResponse | null>(null)
  const [gatewayUptime, setGatewayUptime] = useState('0s')

  // Update gateway uptime every second
  useEffect(() => {
    const update = () => {
      if (gatewayStatus?.uptime_seconds != null) {
        const elapsed = Math.floor(gatewayStatus.uptime_seconds)
        const h = Math.floor(elapsed / 3600)
        const m = Math.floor((elapsed % 3600) / 60)
        const s = elapsed % 60
        setGatewayUptime(h > 0 ? `${h}h ${m}m ${s}s` : m > 0 ? `${m}m ${s}s` : `${s}s`)
      }
    }
    update()
    const interval = setInterval(update, 1000)
    return () => clearInterval(interval)
  }, [gatewayStatus?.uptime_seconds])

  // Poll gateway status every 2 seconds
  useEffect(() => {
    let mounted = true
    const poll = async () => {
      try {
        const s = await fetchGatewayStatus()
        if (mounted) setGatewayStatus(s)
      } catch {
        // ignore polling errors
      }
    }
    poll()
    const interval = setInterval(poll, 2000)
    return () => {
      mounted = false
      clearInterval(interval)
    }
  }, [])

  // Poll queue every 5 seconds
  useEffect(() => {
    let mounted = true
    const poll = async () => {
      try {
        const q = await fetchGatewayQueue()
        if (mounted) setQueue(q)
      } catch {
        // ignore polling errors
      }
    }
    poll()
    const interval = setInterval(poll, 5000)
    return () => {
      mounted = false
      clearInterval(interval)
    }
  }, [])

  const sessionsByProjectEntries = gatewayStatus?.sessions_by_project
    ? Object.entries(gatewayStatus.sessions_by_project)
    : []
  const sessionsByAgentEntries = gatewayStatus?.sessions_by_agent
    ? Object.entries(gatewayStatus.sessions_by_agent)
    : []

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {/* Tab Bar */}
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
            {/* Status Bar */}
            <div className="grid grid-cols-1 md:grid-cols-4 gap-0">
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Status</div>
                <div className="flex items-center gap-2">
                  <span className={`inline-block w-3 h-3 rounded-full ${dispatcherStatus?.running ? 'bg-green-500' : 'bg-red-500'}`} />
                  <span className="font-mono font-semibold dark:text-neutral-200">{dispatcherStatus?.running ? 'Running' : 'Stopped'}</span>
                </div>
              </div>
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Uptime</div>
                <div className="font-mono text-lg font-semibold dark:text-neutral-200">{dispatcherUptime}</div>
              </div>
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Queue Depth</div>
                <div className="font-mono text-lg font-semibold dark:text-neutral-200">{dispatcherStatus?.event_queue_depth ?? '—'}</div>
              </div>
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">WebSocket</div>
                <div className="flex items-center gap-2">
                  <span className={`inline-block w-3 h-3 rounded-full ${isConnected ? 'bg-green-500' : 'bg-red-500'}`} />
                  <span className="font-mono dark:text-neutral-200">{isConnected ? 'Connected' : 'Disconnected'}</span>
                </div>
              </div>
            </div>

            {/* Events Processed */}
            {dispatcherStatus?.events_processed && (
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">Events Processed</div>
                <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-0">
                  {Object.entries(dispatcherStatus.events_processed).map(([key, value]) => (
                    <div key={key} className="text-center">
                      <div className="font-mono text-xl font-bold dark:text-neutral-200">{value}</div>
                      <div className="text-[10px] text-neutral-500 dark:text-amber-muted font-mono truncate uppercase tracking-widest">{key}</div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Live Event Feed */}
            <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
              <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">
                Live Event Feed {dispatcherEvents.length > 0 && <span className="text-xs">({dispatcherEvents.length})</span>}
              </div>
              <div className="h-64 overflow-y-auto space-y-1 font-mono text-xs">
                {dispatcherEvents.length === 0 ? (
                  <p className="text-neutral-400 dark:text-amber-muted italic">Waiting for dispatcher events...</p>
                ) : (
                  [...dispatcherEvents].reverse().map((evt, i) => (
                    <div key={i} className="flex items-center gap-2 py-1 border-b border-gray-200 dark:border-gray-border last:border-0">
                      <span className="text-neutral-400 w-16 shrink-0">{formatTime(evt.timestamp)}</span>
                      <span className={`text-[10px] px-1.5 py-0.5 font-mono ${eventBadgeColor(evt.type)}`}>
                        {evt.type}
                      </span>
                      {evt.story_id && <span className="text-neutral-500 truncate">story: {evt.story_id.substring(0, 8)}</span>}
                    </div>
                  ))
                )}
              </div>
            </div>

            {/* Pipeline Panels */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-0">
              {/* Assignment Pipeline */}
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">Assignment Pipeline</div>
                <div className="space-y-2">
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">Ready Tasks</span>
                    <span className="font-mono font-bold dark:text-neutral-200">—</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">Active Sessions</span>
                    <span className="font-mono font-bold dark:text-neutral-200">—</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">Last Pass</span>
                    <span className="font-mono text-neutral-500 dark:text-neutral-400">—</span>
                  </div>
                </div>
              </div>

              {/* Gate Pipeline */}
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">Gate Pipeline</div>
                <div className="space-y-2">
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">Pending Build Gates</span>
                    <span className="font-mono font-bold dark:text-neutral-200">—</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">Pending Review Gates</span>
                    <span className="font-mono font-bold dark:text-neutral-200">—</span>
                  </div>
                </div>
              </div>

              {/* Staleness Monitor */}
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">Staleness Monitor</div>
                <div className="space-y-2">
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">Stale Sessions</span>
                    <span className="font-mono font-bold dark:text-neutral-200">—</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">Last Check</span>
                    <span className="font-mono text-neutral-500 dark:text-neutral-400">—</span>
                  </div>
                </div>
              </div>
            </div>
          </>
        ) : (
          <>
            {/* Status Bar */}
            <div className="grid grid-cols-1 md:grid-cols-5 gap-0">
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Status</div>
                <div className="flex items-center gap-2">
                  <span className={`inline-block w-3 h-3 rounded-full ${gatewayStatus?.running ? 'bg-green-500' : 'bg-red-500'}`} />
                  <span className="font-mono font-semibold dark:text-neutral-200">{gatewayStatus?.running ? 'Running' : 'Stopped'}</span>
                </div>
              </div>
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Active Sessions</div>
                <div className="font-mono text-lg font-semibold dark:text-neutral-200">{gatewayStatus?.active_sessions ?? '—'}</div>
              </div>
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Queue Depth</div>
                <div className="font-mono text-lg font-semibold dark:text-neutral-200">{gatewayStatus?.queue_depth ?? '—'}</div>
              </div>
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Events Processed</div>
                <div className="font-mono text-lg font-semibold dark:text-neutral-200">{gatewayStatus?.events_processed ?? '—'}</div>
              </div>
              <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
                <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Uptime</div>
                <div className="font-mono text-lg font-semibold dark:text-neutral-200">{gatewayUptime}</div>
              </div>
            </div>

            {/* Sessions by Project */}
            <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
              <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">Sessions by Project</div>
              {sessionsByProjectEntries.length === 0 ? (
                <p className="font-mono text-xs text-neutral-400 dark:text-amber-muted italic">No session data available.</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full font-mono text-xs">
                    <thead>
                      <tr className="border-b border-gray-200 dark:border-gray-border">
                        <th className="text-left py-2 pr-4 text-neutral-500 dark:text-amber-muted uppercase tracking-widest">Project</th>
                        <th className="text-right py-2 text-neutral-500 dark:text-amber-muted uppercase tracking-widest">Sessions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {sessionsByProjectEntries.map(([project, count]) => (
                        <tr key={project} className="border-b border-gray-200 dark:border-gray-border last:border-0">
                          <td className="py-2 pr-4 text-neutral-700 dark:text-neutral-300">{project}</td>
                          <td className="py-2 text-right font-bold dark:text-neutral-200">{count}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>

            {/* Sessions by Agent */}
            <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
              <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">Sessions by Agent</div>
              {sessionsByAgentEntries.length === 0 ? (
                <p className="font-mono text-xs text-neutral-400 dark:text-amber-muted italic">No session data available.</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full font-mono text-xs">
                    <thead>
                      <tr className="border-b border-gray-200 dark:border-gray-border">
                        <th className="text-left py-2 pr-4 text-neutral-500 dark:text-amber-muted uppercase tracking-widest">Agent Type</th>
                        <th className="text-right py-2 text-neutral-500 dark:text-amber-muted uppercase tracking-widest">Sessions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {sessionsByAgentEntries.map(([agent, count]) => (
                        <tr key={agent} className="border-b border-gray-200 dark:border-gray-border last:border-0">
                          <td className="py-2 pr-4 text-neutral-700 dark:text-neutral-300">{agent}</td>
                          <td className="py-2 text-right font-bold dark:text-neutral-200">{count}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>

            {/* Queue Panel */}
            <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
              <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">
                Queue {queue != null && <span className="text-xs">({queue.total})</span>}
              </div>
              {queue == null || queue.jobs.length === 0 ? (
                <p className="font-mono text-xs text-neutral-400 dark:text-amber-muted italic">Queue is empty.</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full font-mono text-xs">
                    <thead>
                      <tr className="border-b border-gray-200 dark:border-gray-border">
                        <th className="text-left py-2 pr-4 text-neutral-500 dark:text-amber-muted uppercase tracking-widest">Project ID</th>
                        <th className="text-left py-2 pr-4 text-neutral-500 dark:text-amber-muted uppercase tracking-widest">Agent Type</th>
                        <th className="text-left py-2 pr-4 text-neutral-500 dark:text-amber-muted uppercase tracking-widest">Task ID</th>
                        <th className="text-left py-2 text-neutral-500 dark:text-amber-muted uppercase tracking-widest">Created At</th>
                      </tr>
                    </thead>
                    <tbody>
                      {queue.jobs.map((job: GatewayJob) => (
                        <tr key={job.id} className="border-b border-gray-200 dark:border-gray-border last:border-0">
                          <td className="py-2 pr-4 text-neutral-700 dark:text-neutral-300">{job.project_id}</td>
                          <td className="py-2 pr-4 text-neutral-700 dark:text-neutral-300">{job.agent_type}</td>
                          <td className="py-2 pr-4 text-neutral-700 dark:text-neutral-300">
                            <span className="font-mono">{job.task_id.substring(0, 8)}</span>
                          </td>
                          <td className="py-2 text-neutral-500 dark:text-amber-muted">{formatTime(job.created_at)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  )
}
