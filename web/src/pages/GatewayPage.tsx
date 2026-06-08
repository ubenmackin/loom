import { useState, useEffect } from 'react'
import { fetchGatewayStatus, fetchGatewayQueue, triggerGatewayAction } from '../api/client'
import type { GatewayStatus, GatewayJob, GatewayQueueResponse } from '../api/client'

function formatTime(ts: string): string {
  const d = new Date(ts)
  return d.toLocaleTimeString()
}

export default function GatewayPage() {
  const [status, setStatus] = useState<GatewayStatus | null>(null)
  const [queue, setQueue] = useState<GatewayQueueResponse | null>(null)
  const [triggerForm, setTriggerForm] = useState({
    event_type: '',
    project_id: '',
    agent_type: '',
    task_id: '',
  })
  const [triggerMessage, setTriggerMessage] = useState('')
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

  // Poll gateway status every 2 seconds
  useEffect(() => {
    let mounted = true
    const poll = async () => {
      try {
        const s = await fetchGatewayStatus()
        if (mounted) setStatus(s)
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

  const handleTrigger = async (e: React.FormEvent) => {
    e.preventDefault()
    setTriggerMessage('')
    try {
      await triggerGatewayAction(triggerForm)
      setTriggerMessage('Action triggered successfully.')
      setTriggerForm({ event_type: '', project_id: '', agent_type: '', task_id: '' })
    } catch (err) {
      setTriggerMessage(`Trigger failed: ${err instanceof Error ? err.message : 'Unknown error'}`)
    }
  }

  const handleTriggerChange = (field: string) => (e: React.ChangeEvent<HTMLInputElement>) => {
    setTriggerForm((prev) => ({ ...prev, [field]: e.target.value }))
  }

  const sessionsByProjectEntries = status?.sessions_by_project
    ? Object.entries(status.sessions_by_project)
    : []
  const sessionsByAgentEntries = status?.sessions_by_agent
    ? Object.entries(status.sessions_by_agent)
    : []

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-border">
        <span className="text-[10px] uppercase tracking-widest font-bold text-neutral-600 dark:text-neutral-300">
          Gateway Dashboard
        </span>
      </div>

      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {/* Status Bar */}
        <div className="grid grid-cols-1 md:grid-cols-5 gap-0">
          <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
            <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Status</div>
            <div className="flex items-center gap-2">
              <span className={`inline-block w-3 h-3 rounded-full ${status?.running ? 'bg-green-500' : 'bg-red-500'}`} />
              <span className="font-mono font-semibold dark:text-neutral-200">{status?.running ? 'Running' : 'Stopped'}</span>
            </div>
          </div>
          <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
            <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Active Sessions</div>
            <div className="font-mono text-lg font-semibold dark:text-neutral-200">{status?.active_sessions ?? '—'}</div>
          </div>
          <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
            <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Queue Depth</div>
            <div className="font-mono text-lg font-semibold dark:text-neutral-200">{status?.queue_depth ?? '—'}</div>
          </div>
          <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
            <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Events Processed</div>
            <div className="font-mono text-lg font-semibold dark:text-neutral-200">{status?.events_processed ?? '—'}</div>
          </div>
          <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
            <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Uptime</div>
            <div className="font-mono text-lg font-semibold dark:text-neutral-200">{uptime}</div>
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

        {/* Manual Trigger */}
        <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-3">
          <div className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-3">Manual Trigger</div>
          <form onSubmit={handleTrigger} className="space-y-3">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3">
              <div>
                <label className="block text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Event Type</label>
                <input
                  type="text"
                  value={triggerForm.event_type}
                  onChange={handleTriggerChange('event_type')}
                  placeholder="e.g. build_event"
                  className="w-full px-2 py-1.5 text-sm font-mono bg-white dark:bg-charcoal-darkest border border-gray-200 dark:border-gray-border text-neutral-900 dark:text-neutral-200 placeholder-neutral-400 dark:placeholder-amber-muted focus:outline-none focus:ring-1 focus:ring-purple-active"
                  required
                />
              </div>
              <div>
                <label className="block text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Project ID</label>
                <input
                  type="text"
                  value={triggerForm.project_id}
                  onChange={handleTriggerChange('project_id')}
                  placeholder="project-123"
                  className="w-full px-2 py-1.5 text-sm font-mono bg-white dark:bg-charcoal-darkest border border-gray-200 dark:border-gray-border text-neutral-900 dark:text-neutral-200 placeholder-neutral-400 dark:placeholder-amber-muted focus:outline-none focus:ring-1 focus:ring-purple-active"
                  required
                />
              </div>
              <div>
                <label className="block text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Agent Type</label>
                <input
                  type="text"
                  value={triggerForm.agent_type}
                  onChange={handleTriggerChange('agent_type')}
                  placeholder="e.g. architect"
                  className="w-full px-2 py-1.5 text-sm font-mono bg-white dark:bg-charcoal-darkest border border-gray-200 dark:border-gray-border text-neutral-900 dark:text-neutral-200 placeholder-neutral-400 dark:placeholder-amber-muted focus:outline-none focus:ring-1 focus:ring-purple-active"
                  required
                />
              </div>
              <div>
                <label className="block text-[10px] uppercase tracking-widest text-neutral-500 dark:text-amber-muted font-mono mb-1">Task ID</label>
                <input
                  type="text"
                  value={triggerForm.task_id}
                  onChange={handleTriggerChange('task_id')}
                  placeholder="task-abc"
                  className="w-full px-2 py-1.5 text-sm font-mono bg-white dark:bg-charcoal-darkest border border-gray-200 dark:border-gray-border text-neutral-900 dark:text-neutral-200 placeholder-neutral-400 dark:placeholder-amber-muted focus:outline-none focus:ring-1 focus:ring-purple-active"
                  required
                />
              </div>
            </div>
            <div className="flex items-center gap-3">
              <button
                type="submit"
                className="px-4 py-1.5 text-sm font-mono font-semibold bg-purple-active text-white hover:opacity-90 transition-opacity"
              >
                Trigger
              </button>
              {triggerMessage && (
                <span className={`font-mono text-xs ${triggerMessage.startsWith('Trigger failed') ? 'text-red-500' : 'text-green-500'}`}>
                  {triggerMessage}
                </span>
              )}
            </div>
          </form>
        </div>
      </div>
    </div>
  )
}