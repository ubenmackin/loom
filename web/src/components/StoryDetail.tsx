import { memo, useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { X, AlertCircle, Trash2 } from 'lucide-react'
import SharpTag from './SharpTag'
import SlideInPanel, { PanelLoading, PanelNotFound } from './SlideInPanel'
import EditableTitle from './EditableTitle'
import FieldLabel from './FieldLabel'

import ConfirmModal from './ConfirmModal'
import {
  fetchStory,
  updateStory,
  deleteStory,
  getUsers,
  fetchSessions,
  createTask,
} from '../api/client'
import type { Story, StoryWithTasks, StatusType, User, Session, AssigneeTypeType } from '../types'
import { statusVariant, STATUS_ORDER } from '../utils/status'

interface StoryDetailProps {
  storyId: string | null
  onClose: () => void
  onOpenTask?: (taskId: string) => void
}

interface StoryDraft {
  title: string
  description: string
  priority: number
  requires_build: boolean
  requires_review: boolean
  status: StatusType
  assigned_to: string
  assignee_type: string
}

function StoryDetail({ storyId, onClose, onOpenTask }: StoryDetailProps) {
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<StoryDraft | null>(null)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const [showDropdown, setShowDropdown] = useState(false)
  const [showAddTask, setShowAddTask] = useState(false)
  const [newTaskTitle, setNewTaskTitle] = useState('')
  const [newTaskType, setNewTaskType] = useState<'code' | 'build' | 'review'>('code')

  const { data, isLoading } = useQuery<StoryWithTasks>({
    queryKey: ['story', storyId],
    queryFn: () => fetchStory(storyId!),
    enabled: !!storyId,
  })

  const story = data?.story
  const tasks = data?.tasks

  const updateMutation = useMutation({
    mutationFn: (data: Partial<Story>) => updateStory(storyId!, data),
    onMutate: async (data) => {
      await queryClient.cancelQueries({ queryKey: ['story', storyId] })
      const previous = queryClient.getQueryData<StoryWithTasks>(['story', storyId])
      if (previous) {
        queryClient.setQueryData(['story', storyId], {
          ...previous,
          story: { ...previous.story, ...data },
        })
      }
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(['story', storyId], context.previous)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['story', storyId] })
      queryClient.invalidateQueries({ queryKey: ['board'] })
    },
  })

  const { data: users = [] } = useQuery<User[]>({
    queryKey: ['users'],
    queryFn: getUsers,
  })

  const { data: sessions = [] } = useQuery<Session[]>({
    queryKey: ['sessions'],
    queryFn: fetchSessions,
  })

  const deleteMutation = useMutation({
    mutationFn: () => deleteStory(storyId!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['board'] })
      onClose()
    },
  })

  const createTaskMutation = useMutation({
    mutationFn: ({ title, task_type }: { title: string; task_type: 'code' | 'build' | 'review' }) =>
      createTask(storyId!, { title, task_type }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['story', storyId] })
      queryClient.invalidateQueries({ queryKey: ['board'] })
      setShowAddTask(false)
      setNewTaskTitle('')
      setNewTaskType('code')
    },
  })

  // ESC key handler — must be before any early return (React hooks order rule)
  useEffect(() => {
    if (!storyId) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [storyId, onClose])

  // Sync draft from story whenever story changes
  useEffect(() => {
    if (story) {
      setDraft({
        title: story.title,
        description: story.description ?? '',
        priority: story.priority,
        requires_build: story.requires_build,
        requires_review: story.requires_review,
        status: story.status,
        assigned_to: story.assigned_to ?? '',
        assignee_type: story.assignee_type ?? '',
      })
    }
  }, [story])

  if (!storyId) return null

  if (isLoading) {
    return <PanelLoading message="Loading story..." />
  }

  if (!story) {
    return <PanelNotFound message="Story not found" />
  }

  const isDirty = Boolean(
    draft &&
      story &&
      (draft.title !== story.title ||
        draft.description !== (story.description ?? '') ||
        draft.priority !== story.priority ||
        draft.requires_build !== story.requires_build ||
        draft.requires_review !== story.requires_review ||
        draft.status !== story.status ||
        draft.assigned_to !== (story.assigned_to ?? '') ||
        draft.assignee_type !== (story.assignee_type ?? ''))
  )

  const handleSave = () => {
    if (!draft || !story || !isDirty) return
    const changes: Partial<Story> = {}
    if (draft.title !== story.title) changes.title = draft.title
    if (draft.description !== (story.description ?? '')) changes.description = draft.description
    if (draft.priority !== story.priority) changes.priority = draft.priority
    if (draft.requires_build !== story.requires_build) changes.requires_build = draft.requires_build
    if (draft.requires_review !== story.requires_review) changes.requires_review = draft.requires_review
    if (draft.status !== story.status) changes.status = draft.status
    if (draft.assigned_to !== (story.assigned_to ?? '')) changes.assigned_to = draft.assigned_to || undefined
    if (draft.assignee_type !== (story.assignee_type ?? '')) changes.assignee_type = (draft.assignee_type || undefined) as AssigneeTypeType | undefined
    updateMutation.mutate(changes)
  }

  const handleCancel = () => {
    if (story) {
      setDraft({
        title: story.title,
        description: story.description ?? '',
        priority: story.priority,
        requires_build: story.requires_build,
        requires_review: story.requires_review,
        status: story.status,
        assigned_to: story.assigned_to ?? '',
        assignee_type: story.assignee_type ?? '',
      })
    }
  }

  const assigneeOptions = [
    ...users.map((u) => ({ id: u.id, name: u.display_name || u.username, type: 'human' as const })),
    ...sessions.map((s) => ({ id: s.id, name: s.id, type: 'session' as const })),
  ]

  const getAssigneeName = (id: string): string => {
    const option = assigneeOptions.find((o) => o.id === id)
    return option?.name ?? id
  }

  const filteredOptions = searchTerm
    ? assigneeOptions.filter((o) => o.name.toLowerCase().includes(searchTerm.toLowerCase()))
    : assigneeOptions

  return (
    <SlideInPanel>
      {/* Header */}
      <div className="sticky top-0 bg-white dark:bg-charcoal-dark border-b border-gray-200 dark:border-gray-border px-4 py-3 z-10">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">
              {story.id}
            </span>
            <SharpTag label="STORY" variant="primary" />
          </div>
          <button
            onClick={onClose}
            className="p-1 rounded-none text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-200 transition-colors"
            aria-label="Close"
          >
            <X size={16} />
          </button>
        </div>

        {/* Editable title */}
        <EditableTitle value={draft?.title ?? story.title} onSave={(title) => setDraft((prev) => (prev ? { ...prev, title } : null))} />
      </div>

      {/* Fields */}
      <div className="px-4 py-4 space-y-5">
        {/* Description */}
        <div>
          <FieldLabel>Description</FieldLabel>
          <textarea
            value={draft?.description ?? story.description ?? ''}
            onChange={(e) => setDraft((prev) => (prev ? { ...prev, description: e.target.value } : null))}
            rows={4}
            className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-3 font-mono text-sm text-neutral-800 dark:text-light-neutral resize-y"
            placeholder="Markdown description..."
          />
        </div>

        {/* Priority */}
        <div>
          <FieldLabel>Priority</FieldLabel>
          <input
            type="number"
            value={draft?.priority ?? story.priority}
            onChange={(e) =>
              setDraft((prev) => (prev ? { ...prev, priority: parseInt(e.target.value, 10) || 0 } : null))
            }
            className="w-20 rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral"
          />
        </div>

        {/* Status */}
        <div>
          <FieldLabel margin="mb-2">Status</FieldLabel>
          <select
            value={draft?.status ?? story.status}
            onChange={(e) => setDraft((prev) => (prev ? { ...prev, status: e.target.value as StatusType } : null))}
            className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral"
          >
            {STATUS_ORDER.map((s) => (
              <option key={s} value={s}>
                {s === 'new' ? 'New' : s === 'ready' ? 'Ready' : s === 'in_progress' ? 'In Progress' : s === 'blocked' ? 'Blocked' : s === 'done' ? 'Done' : s === 'canceled' ? 'Canceled' : s === 'archived' ? 'Archived' : s}
              </option>
            ))}
          </select>
        </div>

        {/* Build toggle */}
        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={draft?.requires_build ?? story.requires_build}
            onChange={(e) => setDraft((prev) => (prev ? { ...prev, requires_build: e.target.checked } : null))}
            className="rounded-none accent-purple-active"
          />
          <SharpTag label="BUILD" variant="amber" />
          <span className="text-xs text-neutral-500 dark:text-neutral-400">
            Requires build
          </span>
        </div>

        {/* Review toggle */}
        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={draft?.requires_review ?? story.requires_review}
            onChange={(e) => setDraft((prev) => (prev ? { ...prev, requires_review: e.target.checked } : null))}
            className="rounded-none accent-purple-active"
          />
          <SharpTag label="REVIEW" variant="success" />
          <span className="text-xs text-neutral-500 dark:text-neutral-400">
            Requires review
          </span>
        </div>

        {/* Assigned to */}
        <div>
          <FieldLabel>Assigned To</FieldLabel>
          <div className="relative">
            {draft?.assigned_to ? (
              <div className="flex items-center gap-2 mb-1">
                <span className="font-mono text-sm text-neutral-800 dark:text-light-neutral">
                  {getAssigneeName(draft.assigned_to)}
                </span>
                <SharpTag
                  label={draft.assignee_type === 'session' ? 'AGENT' : 'USER'}
                  variant={draft.assignee_type === 'session' ? 'amber' : 'primary'}
                />
                <button
                  onClick={() => setDraft((prev) => (prev ? { ...prev, assigned_to: '', assignee_type: '' } : null))}
                  className="text-[10px] uppercase tracking-wider text-red-500 hover:text-red-400 transition-colors"
                >
                  <X size={14} />
                </button>
              </div>
            ) : (
              <>
                <input
                  type="text"
                  value={searchTerm}
                  onChange={(e) => {
                    setSearchTerm(e.target.value)
                    setShowDropdown(true)
                  }}
                  onFocus={() => setShowDropdown(true)}
                  onBlur={() => setTimeout(() => setShowDropdown(false), 200)}
                  placeholder="Search users or agents..."
                  className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral"
                />
                {showDropdown && (
                  <div className="absolute z-20 w-full mt-1 border border-gray-200 dark:border-gray-border bg-white dark:bg-charcoal-dark max-h-48 overflow-y-auto">
                    {filteredOptions.length > 0 ? (
                      filteredOptions.map((opt) => (
                        <button
                          key={opt.id}
                          onMouseDown={() => {
                            setDraft((prev) => (prev ? { ...prev, assigned_to: opt.id, assignee_type: opt.type } : null))
                            setSearchTerm('')
                            setShowDropdown(false)
                          }}
                          className="w-full text-left px-3 py-2 hover:bg-neutral-100 dark:hover:bg-neutral-800 flex items-center gap-2"
                        >
                          <span className="font-mono text-sm text-neutral-800 dark:text-light-neutral">
                            {opt.name}
                          </span>
                          <SharpTag
                            label={opt.type === 'session' ? 'AGENT' : 'USER'}
                            variant={opt.type === 'session' ? 'amber' : 'primary'}
                          />
                        </button>
                      ))
                    ) : (
                      <div className="px-3 py-2 font-mono text-xs text-neutral-400 dark:text-neutral-500">
                        No matches
                      </div>
                    )}
                  </div>
                )}
              </>
            )}
          </div>
        </div>

        {/* Child tasks */}
        <div>
          <FieldLabel margin="mb-2">
            Child Tasks ({tasks?.length ?? 0})
          </FieldLabel>
          {tasks && tasks.length > 0 ? (
            <div className="space-y-2">
              {tasks.map((task) => (
                <button
                  key={task.id}
                  onClick={() => onOpenTask?.(task.id)}
                  className="w-full text-left"
                >
                  <div className="flex items-center gap-2 px-3 py-1 border border-gray-200 dark:border-gray-border hover:bg-neutral-50 dark:hover:bg-neutral-800 transition-colors">
                    <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">
                      {task.id}
                    </span>
                    <span className="text-xs text-neutral-800 dark:text-light-neutral truncate flex-1">
                      {task.title}
                    </span>
                    <SharpTag
                      label={task.status.toUpperCase()}
                      variant={statusVariant(task.status)}
                    />
                  </div>
                </button>
              ))}
            </div>
          ) : (
            <div className="flex items-center gap-2 px-3 py-2 border border-gray-200 dark:border-gray-border">
              <AlertCircle size={12} className="text-neutral-400" />
              <span className="font-mono text-xs text-neutral-400 dark:text-neutral-500">
                No tasks
              </span>
            </div>
          )}

          {/* Add Task inline form */}
          {showAddTask ? (
            <div className="mt-2 p-3 border border-gray-200 dark:border-gray-border space-y-2">
              <input
                type="text"
                value={newTaskTitle}
                onChange={(e) => setNewTaskTitle(e.target.value)}
                placeholder="Task title..."
                className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral"
                autoFocus
              />
              <select
                value={newTaskType}
                onChange={(e) => setNewTaskType(e.target.value as 'code' | 'build' | 'review')}
                className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral"
              >
                <option value="code">CODE</option>
                <option value="build">BUILD</option>
                <option value="review">REVIEW</option>
              </select>
              <div className="flex items-center justify-end gap-2">
                <button
                  onClick={() => {
                    setShowAddTask(false)
                    setNewTaskTitle('')
                    setNewTaskType('code')
                  }}
                  className="px-3 py-1 text-xs font-mono uppercase tracking-wider text-neutral-500 hover:text-neutral-700 dark:hover:text-neutral-300 transition-colors"
                >
                  Cancel
                </button>
                <button
                  onClick={() => {
                    if (newTaskTitle.trim()) {
                      createTaskMutation.mutate({ title: newTaskTitle.trim(), task_type: newTaskType })
                    }
                  }}
                  disabled={!newTaskTitle.trim() || createTaskMutation.isPending}
                  className="px-3 py-1 text-xs font-mono uppercase tracking-wider bg-purple-active text-white rounded-none hover:bg-purple-600 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                >
                  Create
                </button>
              </div>
            </div>
          ) : (
            <button
              onClick={() => setShowAddTask(true)}
              className="mt-2 text-xs font-mono uppercase tracking-wider text-purple-active hover:text-purple-400 transition-colors"
            >
              + Add Task
            </button>
          )}
        </div>

        {/* Save/Cancel */}
        <div className="sticky bottom-0 bg-white dark:bg-charcoal-dark border-t border-gray-200 dark:border-gray-border px-4 py-3 z-10 flex items-center justify-end gap-2">
          <button
            onClick={handleCancel}
            className="px-4 py-1.5 text-sm font-medium text-neutral-600 dark:text-neutral-300 hover:text-neutral-800 dark:hover:text-neutral-100 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={!isDirty}
            className="px-4 py-1.5 text-sm font-medium bg-purple-active text-white rounded-none hover:bg-purple-600 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >
            Save Changes
          </button>
        </div>

        {/* Delete */}
        <div className="border-t border-gray-200 dark:border-gray-border pt-4 mt-4">
          <button
            onClick={() => setShowDeleteConfirm(true)}
            disabled={deleteMutation.isPending}
            className="text-xs font-mono uppercase tracking-wider text-red-500 hover:text-red-400 disabled:opacity-40 transition-colors"
          >
            <Trash2 size={14} className="inline mr-1.5 -mt-0.5" />
            Delete Story
          </button>
        </div>
      </div>

      <ConfirmModal
        open={showDeleteConfirm}
        title="Delete Story"
        message="Are you sure you want to delete this story? This action cannot be undone."
        onConfirm={() => {
          setShowDeleteConfirm(false)
          deleteMutation.mutate()
        }}
        onCancel={() => setShowDeleteConfirm(false)}
      />
    </SlideInPanel>
  )
}

export default memo(StoryDetail)
