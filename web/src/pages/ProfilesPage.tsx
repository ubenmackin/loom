import { useEffect, useState } from 'react'
import {
  fetchProfiles,
  createProfile,
  updateProfile,
  deleteProfile,
  importProfiles,
  fetchProjects,
  type AgentProfile,
} from '../api/client'
import { TaskType, type Project } from '../types'
import TriggerRulesEditor from '../components/TriggerRulesEditor'

interface ProfileCreatePayload {
  name: string
  description?: string
  capabilities?: string
  max_concurrency?: number
  task_types?: string[]
}

export default function ProfilesPage() {
  const [profiles, setProfiles] = useState<AgentProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [editForm, setEditForm] = useState<Partial<AgentProfile>>({})
  const [expandedRulesProfile, setExpandedRulesProfile] = useState<string | null>(null)
  const [showImportModal, setShowImportModal] = useState(false)
  const [projects, setProjects] = useState<Project[]>([])
  const [selectedProjectId, setSelectedProjectId] = useState('')
  const [importing, setImporting] = useState(false)

  useEffect(() => {
    loadProfiles()
  }, [])

  async function loadProfiles() {
    try {
      setLoading(true)
      const data = await fetchProfiles()
      setProfiles(data)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load profiles')
    } finally {
      setLoading(false)
    }
  }

  function startCreate() {
    setCreating(true)
    setEditForm({ name: '', description: '', capabilities: '[]', max_concurrency: 5, task_types: [] })
    setEditingId(null)
  }

  function startEdit(profile: AgentProfile) {
    setEditingId(profile.id)
    setEditForm({ ...profile })
    setCreating(false)
  }

  function cancelEdit() {
    setEditingId(null)
    setCreating(false)
    setEditForm({})
  }

  function toggleRules(profileId: string) {
    setExpandedRulesProfile(expandedRulesProfile === profileId ? null : profileId)
  }

  async function handleSave() {
    try {
      if (creating) {
        if (!editForm.name) {
          setError('Profile name is required')
          return
        }
        const payload: ProfileCreatePayload = {
          name: editForm.name,
          description: editForm.description,
          capabilities: editForm.capabilities,
          max_concurrency: editForm.max_concurrency,
          task_types: editForm.task_types,
        }
        await createProfile(payload)
      } else if (editingId) {
        await updateProfile(editingId, editForm)
      }
      cancelEdit()
      await loadProfiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save profile')
    }
  }

  async function handleDelete(id: string) {
    if (!window.confirm('Are you sure you want to delete this profile?')) return
    try {
      await deleteProfile(id)
      await loadProfiles()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete profile')
    }
  }

  function parseCapabilities(capabilities?: string): string[] {
    if (!capabilities) return []
    try {
      return JSON.parse(capabilities)
    } catch {
      return []
    }
  }

  async function openImportModal() {
    try {
      const allProjects = await fetchProjects()
      setProjects(allProjects)
      setSelectedProjectId(allProjects.length > 0 ? allProjects[0].id : '')
      setShowImportModal(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load projects')
    }
  }

  async function handleImport() {
    if (!selectedProjectId) {
      setError('Please select a project')
      return
    }
    try {
      setImporting(true)
      await importProfiles(selectedProjectId)
      setShowImportModal(false)
      await loadProfiles()
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to import profiles')
    } finally {
      setImporting(false)
    }
  }

  if (loading) {
    return (
      <div className="p-6">
        <div className="text-sm text-slate-500 dark:text-neutral-400">Loading profiles...</div>
      </div>
    )
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-slate-800 dark:text-white">Agent Profiles</h1>
        <div className="flex gap-2">
          <button
            onClick={openImportModal}
            className="px-4 py-2 text-sm font-medium text-purple-700 dark:text-purple-300 bg-transparent border border-purple-600 dark:border-purple-500 hover:bg-purple-50 dark:hover:bg-purple-900/20 rounded-none transition-colors"
          >
            Import from opencode.json
          </button>
          <button
            onClick={startCreate}
            className="px-4 py-2 text-sm font-medium text-white bg-purple-600 hover:bg-purple-700 rounded-none transition-colors"
          >
            + Create Profile
          </button>
        </div>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-400 text-sm">
          {error}
        </div>
      )}

      {/* Create form */}
      {creating && (
        <div className="mb-6 p-4 border border-gray-200 dark:border-gray-border bg-white dark:bg-charcoal-dark">
          <h3 className="text-lg font-semibold mb-4 text-slate-800 dark:text-white">New Profile</h3>
          <ProfileForm form={editForm} onChange={setEditForm} />
          <div className="flex gap-2 mt-4">
            <button onClick={handleSave} className="px-4 py-2 text-sm font-medium text-white bg-purple-600 hover:bg-purple-700 rounded-none transition-colors">
              Save
            </button>
            <button onClick={cancelEdit} className="px-4 py-2 text-sm font-medium text-slate-700 dark:text-neutral-300 bg-gray-100 dark:bg-charcoal-darkest hover:bg-gray-200 dark:hover:bg-gray-800 rounded-none transition-colors">
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Profile cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {profiles.map((profile) => (
          <div
            key={profile.id}
            className="border border-gray-200 dark:border-gray-border bg-white dark:bg-charcoal-dark p-4"
          >
            {editingId === profile.id ? (
              <div>
                <h3 className="text-lg font-semibold mb-4 text-slate-800 dark:text-white">Edit Profile</h3>
                <ProfileForm form={editForm} onChange={setEditForm} />
                <div className="flex gap-2 mt-4">
                  <button onClick={handleSave} className="px-4 py-2 text-sm font-medium text-white bg-purple-600 hover:bg-purple-700 rounded-none transition-colors">
                    Save
                  </button>
                  <button onClick={cancelEdit} className="px-4 py-2 text-sm font-medium text-slate-700 dark:text-neutral-300 bg-gray-100 dark:bg-charcoal-darkest hover:bg-gray-200 dark:hover:bg-gray-800 rounded-none transition-colors">
                    Cancel
                  </button>
                </div>
              </div>
            ) : (
              <div>
                <div className="flex items-start justify-between mb-2">
                  <h3 className="text-lg font-semibold text-slate-800 dark:text-white">{profile.name}</h3>
                  <div className="flex gap-1">
                    <button
                      onClick={() => startEdit(profile)}
                      className="px-2 py-1 text-xs font-medium text-slate-600 dark:text-neutral-400 hover:text-purple-600 dark:hover:text-purple-400 transition-colors"
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => handleDelete(profile.id)}
                      className="px-2 py-1 text-xs font-medium text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-300 transition-colors"
                    >
                      Delete
                    </button>
                  </div>
                </div>
                {profile.description && (
                  <p className="text-sm text-slate-600 dark:text-neutral-400 mb-3">{profile.description}</p>
                )}
                <div className="flex flex-wrap gap-1 mb-3">
                  {/* Task type badges */}
                  {(profile.task_types || []).map((tt) => (
                    <span
                      key={tt}
                      className="px-2 py-0.5 text-xs font-mono bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300"
                    >
                      {tt}
                    </span>
                  ))}
                  {/* Capabilities badges */}
                  {parseCapabilities(profile.capabilities).map((cap) => (
                    <span
                      key={cap}
                      className="px-2 py-0.5 text-xs font-mono bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300"
                    >
                      {cap}
                    </span>
                  ))}
                </div>
                <div className="flex items-center gap-2 text-sm text-slate-600 dark:text-neutral-400">
                  <span>Max concurrency:</span>
                  <span className="font-semibold text-slate-800 dark:text-white">{profile.max_concurrency}</span>
                </div>
                <button
                  onClick={() => toggleRules(profile.id)}
                  className="mt-3 w-full px-3 py-1.5 text-xs font-medium text-purple-700 dark:text-purple-300 bg-purple-50 dark:bg-purple-900/20 hover:bg-purple-100 dark:hover:bg-purple-900/40 border border-purple-200 dark:border-purple-800 transition-colors"
                >
                  {expandedRulesProfile === profile.id ? 'Hide Trigger Rules' : 'Trigger Rules'}
                </button>
                {expandedRulesProfile === profile.id && (
                  <div className="mt-3">
                    <TriggerRulesEditor profileId={profile.id} />
                  </div>
                )}
              </div>
            )}
          </div>
        ))}
      </div>

      {profiles.length === 0 && !creating && (
        <div className="text-center py-12 text-slate-500 dark:text-neutral-400">
          No agent profiles found. Create one to get started.
        </div>
      )}

      {/* Import confirmation modal */}
      {showImportModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="bg-white dark:bg-charcoal-dark border border-gray-200 dark:border-gray-border p-6 max-w-md w-full mx-4">
            <h3 className="text-lg font-semibold mb-2 text-slate-800 dark:text-white">Import Profiles</h3>
            <p className="text-sm text-slate-600 dark:text-neutral-400 mb-4">
              This will read <code className="font-mono text-purple-600 dark:text-purple-400">opencode.json</code> from the selected project's repository and create or update agent profiles.
            </p>

            <label className="block text-sm font-medium text-slate-700 dark:text-neutral-300 mb-1">
              Project
            </label>
            {projects.length === 0 ? (
              <p className="text-sm text-slate-500 dark:text-neutral-400 mb-4">No projects available.</p>
            ) : (
              <select
                value={selectedProjectId}
                onChange={(e) => setSelectedProjectId(e.target.value)}
                className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-darkest text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500 mb-4"
              >
                {projects.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            )}

            <div className="flex gap-2 justify-end">
              <button
                onClick={() => setShowImportModal(false)}
                className="px-4 py-2 text-sm font-medium text-slate-700 dark:text-neutral-300 bg-gray-100 dark:bg-charcoal-darkest hover:bg-gray-200 dark:hover:bg-gray-800 rounded-none transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleImport}
                disabled={importing || projects.length === 0}
                className="px-4 py-2 text-sm font-medium text-white bg-purple-600 hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed rounded-none transition-colors"
              >
                {importing ? 'Importing...' : 'Import'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function ProfileForm({
  form,
  onChange,
}: {
  form: Partial<AgentProfile>
  onChange: (f: Partial<AgentProfile>) => void
}) {
  const taskTypeOptions = [
    { label: 'Code', value: TaskType.Code },
    { label: 'Build', value: TaskType.Build },
    { label: 'Review', value: TaskType.Review },
    { label: 'Planning', value: TaskType.Planning },
  ]

  function toggleTaskType(value: string) {
    const current = form.task_types || []
    if (current.includes(value)) {
      onChange({ ...form, task_types: current.filter((t) => t !== value) })
    } else {
      onChange({ ...form, task_types: [...current, value] })
    }
  }

  return (
    <div className="space-y-4">
      <div>
        <label className="block text-sm font-medium text-slate-700 dark:text-neutral-300 mb-1">Name</label>
        <input
          type="text"
          value={form.name || ''}
          onChange={(e) => onChange({ ...form, name: e.target.value })}
          className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-darkest text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500"
          placeholder="e.g., Planner"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-slate-700 dark:text-neutral-300 mb-1">Description</label>
        <textarea
          value={form.description || ''}
          onChange={(e) => onChange({ ...form, description: e.target.value })}
          className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-darkest text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500"
          rows={2}
          placeholder="Description of this agent type"
        />
      </div>

      {/* Task Types Section */}
      <div>
        <label className="block text-sm font-medium text-slate-700 dark:text-neutral-300 mb-2">
          Task Types
        </label>
        <div className="space-y-2">
          {taskTypeOptions.map((opt) => (
            <label
              key={opt.value}
              className="flex items-center gap-2 cursor-pointer text-sm text-slate-700 dark:text-neutral-300"
            >
              <input
                type="checkbox"
                checked={(form.task_types || []).includes(opt.value)}
                onChange={() => toggleTaskType(opt.value)}
                className="rounded border-gray-300 dark:border-gray-border text-purple-600 focus:ring-purple-500"
              />
              {opt.label}
            </label>
          ))}
        </div>
      </div>

      {/* Keep existing capabilities input as "Additional Capabilities" */}
      <div>
        <label className="block text-sm font-medium text-slate-700 dark:text-neutral-300 mb-1">
          Additional Capabilities <span className="text-xs text-slate-400">(JSON array, for advanced metadata)</span>
        </label>
        <input
          type="text"
          value={form.capabilities || '[]'}
          onChange={(e) => onChange({ ...form, capabilities: e.target.value })}
          className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-border bg-white dark:bg-charcoal-darkest text-slate-800 dark:text-white rounded-none focus:outline-none focus:ring-1 focus:ring-purple-500 font-mono"
          placeholder='["story_planning"]'
        />
      </div>

      {/* Max Concurrency */}
      <div>
        <label className="block text-sm font-medium text-slate-700 dark:text-neutral-300 mb-1">
          Max Concurrency: <span className="font-semibold">{form.max_concurrency ?? 5}</span>
        </label>
        <input
          type="range"
          min={1}
          max={10}
          value={form.max_concurrency ?? 5}
          onChange={(e) => onChange({ ...form, max_concurrency: parseInt(e.target.value) })}
          className="w-full"
        />
        <div className="flex justify-between text-xs text-slate-400">
          <span>1</span>
          <span>10</span>
        </div>
      </div>
    </div>
  )
}
