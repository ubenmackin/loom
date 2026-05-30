import { memo, useState, useEffect, useRef, useMemo, useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { X, AlertCircle, Trash2, ChevronDown } from 'lucide-react'
import SharpTag from './SharpTag'
import SlideInPanel, { PanelLoading, PanelNotFound } from './SlideInPanel'
import EditableTitle from './EditableTitle'
import FieldLabel from './FieldLabel'
import CommentThread from './CommentThread'
import AssigneeSelector from './AssigneeSelector'

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
import { statusVariant, STATUS_ORDER, STATUS_LABELS } from '../utils/status'
import { useWorkItemDraft } from '../hooks/useWorkItemDraft'

interface StoryDetailProps {
  storyId: string | null
  onClose: () => void
  onOpenTask?: (taskId: string) => void
}

interface StoryDraft {
  title: string
  description: string
  requires_build: boolean
  requires_review: boolean
  status: StatusType
  assigned_to: string
  assignee_type: AssigneeTypeType | ''
  sort_order: number
}

function StoryDetail({ storyId, onClose, onOpenTask }: StoryDetailProps) {
  const queryClient = useQueryClient()
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [showAddTask, setShowAddTask] = useState(false)
  const [newTaskTitle, setNewTaskTitle] = useState('')
  const [newTaskType, setNewTaskType] = useState<'code' | 'build' | 'review'>('code')
  const [showSaveDropdown, setShowSaveDropdown] = useState(false)
  const [showGenerateModal, setShowGenerateModal] = useState(false)
  const saveDropdownRef = useRef<HTMLDivElement>(null)

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

  const serverData = useMemo(
    () =>
      story
        ? ({
            title: story.title,
            description: story.description ?? '',
            requires_build: story.requires_build,
            requires_review: story.requires_review,
            status: story.status,
            assigned_to: story.assigned_to ?? '',
            assignee_type: story.assignee_type ?? '',
            sort_order: story.sort_order,
          } as StoryDraft)
        : null,
    [story],
  )

  const { draft, setDraft, isDirty, computeChanges, reset } = useWorkItemDraft(serverData, [
    'title',
    'description',
    'requires_build',
    'requires_review',
    'status',
    'assigned_to',
    'assignee_type',
  ])

  // Click-outside handler for save dropdown
  useEffect(() => {
    if (!showSaveDropdown) return
    const handleClickOutside = (e: MouseEvent) => {
      if (saveDropdownRef.current && !saveDropdownRef.current.contains(e.target as Node)) {
        setShowSaveDropdown(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [showSaveDropdown])

  const handleCancel = useCallback(() => {
    reset()
    onClose()
  }, [reset, onClose])

  const handleAssigneeChange = useCallback(
    (assigned_to: string, assignee_type: string) => {
      setDraft((prev) => (prev ? { ...prev, assigned_to, assignee_type: assignee_type as AssigneeTypeType | '' } : null))
    },
    [setDraft],
  )

  if (!storyId) return null

  if (isLoading) {
    return <PanelLoading message="Loading story..." />
  }

  if (!story) {
    return <PanelNotFound message="Story not found" />
  }

  const handleSave = () => {
    const changes = computeChanges()
    if (!changes) return
    setShowSaveDropdown(false)
    // Convert draft types to API types — empty string assignee_type means "unassign"
    const apiChanges = { ...changes } as Record<string, unknown>
    if (apiChanges.assignee_type === '') {
      apiChanges.assignee_type = undefined
    }
    updateMutation.mutate(apiChanges as Partial<Story>)
  }

  const handleSaveAndClose = () => {
    const changes = computeChanges()
    if (!changes) return
    const apiChanges = { ...changes } as Record<string, unknown>
    if (apiChanges.assignee_type === '') {
      apiChanges.assignee_type = undefined
    }
    updateMutation.mutate(apiChanges as Partial<Story>, {
      onSuccess: () => {
        onClose()
      },
    })
    setShowSaveDropdown(false)
  }

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
                {STATUS_LABELS[s] ?? s}
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
          <AssigneeSelector
            value={draft?.assigned_to ?? ''}
            assigneeType={draft?.assignee_type ?? ''}
            users={users}
            sessions={sessions}
            onChange={handleAssigneeChange}
          />
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

          {/* Generate Tasks — available when story is in "new" or "ready" status */}
          {(story.status === 'new' || story.status === 'ready') && (
            <div className="mt-3">
              <button
                onClick={() => setShowGenerateModal(true)}
                className="text-xs font-mono uppercase tracking-wider text-amber-600 dark:text-amber-400 hover:text-amber-500 transition-colors"
              >
                + Generate Tasks
              </button>
            </div>
          )}
        </div>

        {/* Generate Tasks modal */}
        {showGenerateModal && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
            <div className="mx-4 w-full max-w-md border border-gray-200 dark:border-gray-border bg-white dark:bg-charcoal-dark p-6 shadow-xl">
              <h3 className="text-sm font-semibold text-neutral-800 dark:text-light-neutral mb-3">
                Generate Tasks
              </h3>
              <p className="text-xs text-neutral-600 dark:text-neutral-400 leading-relaxed mb-4">
                Your coding agent will generate tasks from this story automatically.
                The agent will call the <code className="font-mono text-amber-600 dark:text-amber-400">/stories/:id/generate-tasks</code> endpoint
                with a breakdown of tasks. This button is a placeholder — the actual
                generation is triggered by your agent via the REST API.
              </p>
              <div className="flex justify-end">
                <button
                  onClick={() => setShowGenerateModal(false)}
                  className="px-4 py-1.5 text-xs font-mono uppercase tracking-wider bg-purple-active text-white rounded-none hover:bg-purple-600 transition-colors"
                >
                  Got it
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Activity & Comments */}
        <CommentThread workItemId={storyId} workItemType="story" />

        {/* Save/Cancel */}
        <div className="sticky bottom-0 bg-white dark:bg-charcoal-dark border-t border-gray-200 dark:border-gray-border px-4 py-3 z-10 flex items-center justify-end gap-2">
          <button
            onClick={handleCancel}
            className="px-4 py-1.5 text-sm font-medium text-neutral-600 dark:text-neutral-300 hover:text-neutral-800 dark:hover:text-neutral-100 transition-colors"
          >
            Cancel
          </button>
          <div ref={saveDropdownRef} className="relative flex">
            <button
              onClick={handleSave}
              disabled={!isDirty}
              className="rounded-l-md bg-purple-active px-4 py-1.5 text-sm font-medium text-white hover:bg-purple-600 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              Save
            </button>
            <button
              onClick={() => setShowSaveDropdown(!showSaveDropdown)}
              disabled={!isDirty}
              className="rounded-r-md border-l border-purple-600 bg-purple-active px-2 py-1.5 text-white hover:bg-purple-600 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronDown className="h-4 w-4" />
            </button>
            {showSaveDropdown && (
              <div className="absolute bottom-full right-0 mb-1 w-40 rounded-md border border-gray-200 dark:border-gray-border bg-white dark:bg-charcoal-dark py-1 shadow-lg z-20">
                <button
                  onClick={handleSaveAndClose}
                  disabled={!isDirty}
                  className="flex w-full items-center px-4 py-2 text-left text-sm text-gray-700 dark:text-light-neutral hover:bg-gray-100 dark:hover:bg-neutral-800 disabled:opacity-50"
                >
                  Save & Close
                </button>
              </div>
            )}
          </div>
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

// memo is intentionally applied even though storyId changes on every click,
// because the component still benefits from referential stability on the
// onClose/onOpenTask props, and the outer SlideInPanel transition animation
// relies on consistent function references during the mount/unmount lifecycle.
export default memo(StoryDetail)
