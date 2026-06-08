import { useCallback, useEffect, useState } from 'react'
import {
  fetchRulesByProfile,
  createRule,
  updateRule,
  deleteRule,
  type TriggerRule,
} from '../api/client'

interface Props {
  profileId: string
}

const ACTION_OPTIONS = ['create_session', 'resume_session', 'assign_task', 'noop']

export default function TriggerRulesEditor({ profileId }: Props) {
  const [rules, setRules] = useState<TriggerRule[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [savingId, setSavingId] = useState<string | null>(null)
  const [newRule, setNewRule] = useState<{ event_type: string; action: string; priority: number; enabled: boolean }>({
    event_type: '',
    action: 'create_session',
    priority: 0,
    enabled: true,
  })

  const loadRules = useCallback(async () => {
    try {
      setLoading(true)
      const data = await fetchRulesByProfile(profileId)
      setRules(data)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load trigger rules')
    } finally {
      setLoading(false)
    }
  }, [profileId])

  useEffect(() => {
    loadRules()
  }, [loadRules])

  async function handleUpdate(rule: TriggerRule) {
    setSavingId(rule.id)
    try {
      await updateRule(profileId, rule.id, {
        event_type: rule.event_type,
        action: rule.action,
        priority: rule.priority,
        enabled: rule.enabled,
      })
      await loadRules()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update rule')
    } finally {
      setSavingId(null)
    }
  }

  async function handleDelete(ruleId: string) {
    if (!window.confirm('Delete this trigger rule?')) return
    try {
      await deleteRule(profileId, ruleId)
      await loadRules()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete rule')
    }
  }

  async function handleAdd() {
    if (!newRule.event_type || !newRule.action) {
      setError('Event type and action are required')
      return
    }
    try {
      await createRule(profileId, newRule)
      setNewRule({ event_type: '', action: 'create_session', priority: 0, enabled: true })
      await loadRules()
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create rule')
    }
  }

  if (loading) {
    return <div className="text-sm text-slate-500 dark:text-neutral-400 py-4">Loading trigger rules...</div>
  }

  return (
    <div className="border border-gray-200 dark:border-gray-border bg-white dark:bg-charcoal-dark">
      <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-border">
        <h3 className="text-sm font-semibold text-slate-800 dark:text-white">Trigger Rules</h3>
        <p className="text-xs text-slate-500 dark:text-neutral-400 mt-0.5">
          Configure which events trigger which actions for this agent type
        </p>
      </div>

      {error && (
        <div className="mx-4 mt-3 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-400 text-xs">
          {error}
        </div>
      )}

      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-200 dark:border-gray-border bg-gray-50 dark:bg-charcoal-darkest">
              <th className="px-4 py-2 text-left text-xs font-medium text-slate-500 dark:text-neutral-400 uppercase">Event Type</th>
              <th className="px-4 py-2 text-left text-xs font-medium text-slate-500 dark:text-neutral-400 uppercase">Action</th>
              <th className="px-4 py-2 text-left text-xs font-medium text-slate-500 dark:text-neutral-400 uppercase">Priority</th>
              <th className="px-4 py-2 text-center text-xs font-medium text-slate-500 dark:text-neutral-400 uppercase">Enabled</th>
              <th className="px-4 py-2 text-right text-xs font-medium text-slate-500 dark:text-neutral-400 uppercase">Actions</th>
            </tr>
          </thead>
          <tbody>
            {rules.map((rule) => (
              <RuleRow
                key={rule.id}
                rule={rule}
                saving={savingId === rule.id}
                onUpdate={handleUpdate}
                onDelete={handleDelete}
              />
            ))}
            {rules.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-sm text-slate-400 dark:text-neutral-500">
                  No trigger rules configured. Add one below.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Add new rule form */}
      <div className="px-4 py-3 border-t border-gray-200 dark:border-gray-border bg-gray-50 dark:bg-charcoal-darkest">
        <div className="flex items-center gap-2 flex-wrap">
          <input
            type="text"
            value={newRule.event_type}
            onChange={(e) => setNewRule({ ...newRule, event_type: e.target.value })}
            placeholder="event_type (e.g., task_completed)"
            className="flex-1 min-w-[140px] px-2 py-1.5 text-xs border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-dark text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500 font-mono"
          />
          <select
            value={newRule.action}
            onChange={(e) => setNewRule({ ...newRule, action: e.target.value })}
            className="px-2 py-1.5 text-xs border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-dark text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500"
          >
            {ACTION_OPTIONS.map((opt) => (
              <option key={opt} value={opt}>{opt}</option>
            ))}
          </select>
          <input
            type="number"
            value={newRule.priority}
            onChange={(e) => setNewRule({ ...newRule, priority: parseInt(e.target.value) || 0 })}
            className="w-16 px-2 py-1.5 text-xs border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-dark text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500"
            placeholder="0"
          />
          <label className="flex items-center gap-1 text-xs text-slate-600 dark:text-neutral-400">
            <input
              type="checkbox"
              checked={newRule.enabled}
              onChange={(e) => setNewRule({ ...newRule, enabled: e.target.checked })}
              className="rounded"
            />
            Enabled
          </label>
          <button
            onClick={handleAdd}
            className="px-3 py-1.5 text-xs font-medium text-white bg-purple-600 hover:bg-purple-700 rounded-none transition-colors"
          >
            Add
          </button>
        </div>
      </div>
    </div>
  )
}

// ── Rule Row ─────────────────────────────────────────────────────────────

function RuleRow({
  rule,
  saving,
  onUpdate,
  onDelete,
}: {
  rule: TriggerRule
  saving: boolean
  onUpdate: (r: TriggerRule) => void
  onDelete: (id: string) => void
}) {
  const [editing, setEditing] = useState(false)
  const [edit, setEdit] = useState(rule)

  function handleSave() {
    onUpdate(edit)
    setEditing(false)
  }

  function handleCancel() {
    setEdit(rule)
    setEditing(false)
  }

  if (editing) {
    return (
      <tr className="border-b border-gray-100 dark:border-gray-border">
        <td className="px-4 py-2">
          <input
            type="text"
            value={edit.event_type}
            onChange={(e) => setEdit({ ...edit, event_type: e.target.value })}
            className="w-full px-2 py-1 text-xs border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-darkest text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500 font-mono"
          />
        </td>
        <td className="px-4 py-2">
          <select
            value={edit.action}
            onChange={(e) => setEdit({ ...edit, action: e.target.value })}
            className="w-full px-2 py-1 text-xs border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-darkest text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500"
          >
            {ACTION_OPTIONS.map((opt) => (
              <option key={opt} value={opt}>{opt}</option>
            ))}
          </select>
        </td>
        <td className="px-4 py-2">
          <input
            type="number"
            value={edit.priority}
            onChange={(e) => setEdit({ ...edit, priority: parseInt(e.target.value) || 0 })}
            className="w-16 px-2 py-1 text-xs border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-darkest text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500"
          />
        </td>
        <td className="px-4 py-2 text-center">
          <input
            type="checkbox"
            checked={edit.enabled}
            onChange={(e) => setEdit({ ...edit, enabled: e.target.checked })}
            className="rounded"
          />
        </td>
        <td className="px-4 py-2 text-right">
          <div className="flex items-center justify-end gap-1">
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-2 py-1 text-xs font-medium text-white bg-purple-600 hover:bg-purple-700 rounded-none transition-colors disabled:opacity-50"
            >
              {saving ? '...' : 'Save'}
            </button>
            <button
              onClick={handleCancel}
              className="px-2 py-1 text-xs font-medium text-slate-600 dark:text-neutral-400 hover:text-slate-800 dark:hover:text-white transition-colors"
            >
              Cancel
            </button>
          </div>
        </td>
      </tr>
    )
  }

  return (
    <tr className="border-b border-gray-100 dark:border-gray-border hover:bg-gray-50 dark:hover:bg-charcoal-darkest/50 transition-colors">
      <td className="px-4 py-2 font-mono text-xs text-slate-700 dark:text-neutral-300">{rule.event_type}</td>
      <td className="px-4 py-2 font-mono text-xs text-slate-600 dark:text-neutral-400">{rule.action}</td>
      <td className="px-4 py-2 text-xs text-slate-600 dark:text-neutral-400">{rule.priority}</td>
      <td className="px-4 py-2 text-center">
        <span className={`inline-block w-2 h-2 rounded-full ${rule.enabled ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'}`} />
      </td>
      <td className="px-4 py-2 text-right">
        <div className="flex items-center justify-end gap-1">
          <button
            onClick={() => { setEdit(rule); setEditing(true) }}
            className="px-2 py-1 text-xs font-medium text-slate-600 dark:text-neutral-400 hover:text-purple-600 dark:hover:text-purple-400 transition-colors"
          >
            Edit
          </button>
          <button
            onClick={() => onDelete(rule.id)}
            className="px-2 py-1 text-xs font-medium text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-300 transition-colors"
          >
            Delete
          </button>
        </div>
      </td>
    </tr>
  )
}
