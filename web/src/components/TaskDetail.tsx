import { memo, useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { X, ChevronDown, Trash2 } from 'lucide-react'
import SharpTag from './SharpTag'
import CommentThread from './CommentThread'
import ConfirmModal from './ConfirmModal'
import SlideInPanel, { PanelLoading, PanelNotFound } from './SlideInPanel'
import EditableTitle from './EditableTitle'
import FieldLabel from './FieldLabel'
import {
  fetchTask,
  fetchStory,
  updateTask,
  updateTaskStatus,
  fetchBlockers,
  addDependency,
  removeDependency,
  createTask,
  deleteTask,
  getUsers,
  fetchSessions,
} from '../api/client'
import type { Task, TaskTypeType, User, Session, StoryWithTasks, StatusType, AssigneeTypeType, TaskDetailResponse } from '../types'
import { TaskType, AssigneeType } from '../types'
import { STATUS_ORDER } from '../utils/status'
import { taskTypeLabel, taskTypeVariant } from '../utils/taskType'

interface TaskDetailProps {
  taskId: string | null
  onClose: () => void
}

interface TaskDraft {
  title: string
  description: string
  status: StatusType
  assigned_to: string
  assignee_type: string
}

function TaskDetail({ taskId, onClose }: TaskDetailProps) {
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<TaskDraft | null>(null)
  const [createTitle, setCreateTitle] = useState('')
  const [createType, setCreateType] = useState<TaskTypeType>(TaskType.Code)
  const [searchTerm, setSearchTerm] = useState('')
  const [showDropdown, setShowDropdown] = useState(false)
  const [showSaveDropdown, setShowSaveDropdown] = useState(false)
  const [showDepDropdown, setShowDepDropdown] = useState(false)
  const saveDropdownRef = useRef<HTMLDivElement>(null)

  const prevTaskId = useRef(taskId)

  useEffect(() => {
    if (taskId !== prevTaskId.current) {
      prevTaskId.current = taskId
      if (typeof taskId === 'string' && taskId.startsWith('new-task-')) {
        setCreateTitle('')
        setCreateType(TaskType.Code)
      }
    }
  }, [taskId])

  const isCreateMode = typeof taskId === 'string' && taskId.startsWith('new-task-') && taskId.length > 'new-task-'.length
  const createStoryId = isCreateMode ? taskId.slice('new-task-'.length) : null

  // ESC key handler
  useEffect(() => {
    if (!taskId) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [taskId, onClose])

  const { data, isLoading } = useQuery<TaskDetailResponse>({
    queryKey: ['task', taskId],
    queryFn: () => fetchTask(taskId!),
    enabled: !!taskId && !isCreateMode,
  })

  const task = data?.task

  const { data: blockers = [] } = useQuery({
    queryKey: ['blockers', taskId],
    queryFn: () => fetchBlockers(taskId!),
    enabled: !!taskId && !isCreateMode,
  })

  const { data: users = [] } = useQuery<User[]>({
    queryKey: ['users'],
    queryFn: getUsers,
  })

  const { data: sessions = [] } = useQuery<Session[]>({
    queryKey: ['sessions'],
    queryFn: fetchSessions,
  })

  const { data: storyData } = useQuery<StoryWithTasks>({
    queryKey: ['story', task?.story_id],
    queryFn: () => fetchStory(task!.story_id),
    enabled: !!task?.story_id,
  })

  const storyTasks = storyData?.tasks ?? []

  const updateMutation = useMutation({
    mutationFn: (updateData: Partial<Task>) => updateTask(taskId!, updateData),
    onMutate: async (updateData) => {
      await queryClient.cancelQueries({ queryKey: ['task', taskId] })
      const previous = queryClient.getQueryData<TaskDetailResponse>(['task', taskId])
      if (previous) {
        queryClient.setQueryData(['task', taskId], {
          ...previous,
          task: { ...previous.task, ...updateData },
        })
      }
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(['task', taskId], context.previous)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['task', taskId] })
      queryClient.invalidateQueries({ queryKey: ['board'] })
      if (task?.story_id) {
        queryClient.invalidateQueries({ queryKey: ['story', task.story_id] })
      }
    },
  })

  const statusMutation = useMutation({
    mutationFn: (status: string) => updateTaskStatus(taskId!, status),
    onMutate: async (newStatus) => {
      await queryClient.cancelQueries({ queryKey: ['task', taskId] })
      const previous = queryClient.getQueryData<TaskDetailResponse>(['task', taskId])
      if (previous) {
        queryClient.setQueryData(['task', taskId], {
          ...previous,
          task: { ...previous.task, status: newStatus },
        })
      }
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(['task', taskId], context.previous)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['task', taskId] })
      queryClient.invalidateQueries({ queryKey: ['board'] })
    },
  })

  const addDepMutation = useMutation({
    mutationFn: (depId: string) => addDependency(taskId!, depId),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['blockers', taskId] })
    },
  })

  const removeDepMutation = useMutation({
    mutationFn: (depId: string) => removeDependency(taskId!, depId),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['blockers', taskId] })
    },
  })

  const createMutation = useMutation({
    mutationFn: ({ title, task_type }: { title: string; task_type: TaskTypeType }) => {
      if (!createStoryId) throw new Error('No story ID provided for task creation')
      return createTask(createStoryId, { title, task_type })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['board'] })
      if (createStoryId) {
        queryClient.invalidateQueries({ queryKey: ['story', createStoryId] })
      }
      onClose()
    },
    onError: (error) => {
      console.error('Failed to create task:', error)
    },
  })

  // Sync draft from task whenever task changes
  useEffect(() => {
    if (task) {
      setDraft({
        title: task.title,
        description: task.description ?? '',
        status: task.status,
        assigned_to: task.assigned_to ?? '',
        assignee_type: task.assignee_type ?? '',
      })
    }
  }, [task])

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

  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  const deleteMutation = useMutation({
    mutationFn: () => deleteTask(taskId!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['board'] })
      if (task?.story_id) {
        queryClient.invalidateQueries({ queryKey: ['story', task.story_id] })
      }
      onClose()
    },
  })

  if (!taskId) return null

  // ── CREATE MODE ───────────────────────────────────────────────────────────
  if (isCreateMode) {
    return (
      <SlideInPanel>
        {/* Header */}
        <div className="sticky top-0 bg-white dark:bg-charcoal-dark border-b border-gray-200 dark:border-gray-border px-4 py-3 z-10">
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted uppercase tracking-widest">
              Create Task
            </span>
            <button
              onClick={onClose}
              className="p-1 rounded-none text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-200 transition-colors"
              aria-label="Close"
            >
              <X size={16} />
            </button>
          </div>
        </div>

        {/* Form */}
        <div className="px-4 py-4 space-y-5">
          {/* Title input */}
          <div>
            <FieldLabel>Title</FieldLabel>
            <input
              type="text"
              value={createTitle}
              onChange={(e) => setCreateTitle(e.target.value)}
              autoFocus
              placeholder="Task title..."
              className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral"
              onKeyDown={(e) => {
                if (e.key === 'Enter' && createTitle.trim() && !createMutation.isPending) {
                  createMutation.mutate({ title: createTitle.trim(), task_type: createType })
                }
                if (e.key === 'Escape') {
                  onClose()
                }
              }}
            />
          </div>

          {/* Task type selector */}
          <div>
            <FieldLabel>Task Type</FieldLabel>
            <div className="flex gap-2">
              {(['code', 'build', 'review'] as const).map((type) => (
                <button
                  key={type}
                  onClick={() => setCreateType(type)}
                  className={`px-3 py-1.5 border text-xs font-mono uppercase tracking-wider transition-colors ${
                    createType === type
                      ? 'border-neutral-800 dark:border-light-neutral text-neutral-800 dark:text-light-neutral bg-neutral-100 dark:bg-neutral-800'
                      : 'border-gray-200 dark:border-gray-border text-neutral-400 dark:text-neutral-500 hover:border-neutral-400 dark:hover:border-neutral-500'
                  }`}
                >
                  {taskTypeLabel(type)}
                </button>
              ))}
            </div>
          </div>

          {/* Create button */}
          <button
            onClick={() => createMutation.mutate({ title: createTitle.trim(), task_type: createType })}
            disabled={!createTitle.trim() || createMutation.isPending}
            className="w-full py-2 rounded-none border border-neutral-800 dark:border-light-neutral text-neutral-800 dark:text-light-neutral text-xs font-bold uppercase tracking-wider hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {createMutation.isPending ? 'Creating...' : 'Create Task'}
          </button>
        </div>
      </SlideInPanel>
    )
  }

  if (isLoading) {
    return <PanelLoading message="Loading task..." />
  }

  if (!task) {
    return <PanelNotFound message="Task not found" />
  }

  // ── Derived state ──────────────────────────────────────────────────────────

  const isDirty = Boolean(
    draft &&
      task &&
      (draft.title !== task.title ||
        draft.description !== (task.description ?? '') ||
        draft.status !== task.status ||
        draft.assigned_to !== (task.assigned_to ?? '') ||
        draft.assignee_type !== (task.assignee_type ?? ''))
  )

  const computeChanges = (): Partial<Task> | null => {
    if (!draft || !task || !isDirty) return null
    const changes: Partial<Task> = {}
    if (draft.title !== task.title) changes.title = draft.title
    if (draft.description !== (task.description ?? '')) changes.description = draft.description
    if (draft.status !== task.status) changes.status = draft.status
    if (draft.assigned_to !== (task.assigned_to ?? '')) changes.assigned_to = draft.assigned_to
    if (draft.assignee_type !== (task.assignee_type ?? ''))
      changes.assignee_type = draft.assignee_type as AssigneeTypeType | undefined
    return changes
  }

  const handleSave = () => {
    const changes = computeChanges()
    if (!changes) return
    setShowSaveDropdown(false)

    // Extract status change for the dedicated status endpoint
    const statusChange = changes.status
    delete changes.status

    if (Object.keys(changes).length > 0) {
      updateMutation.mutate(changes)
    }
    if (statusChange) {
      statusMutation.mutate(statusChange)
    }
  }

  const handleSaveAndClose = () => {
    const changes = computeChanges()
    if (!changes) return
    setShowSaveDropdown(false)

    // Extract status change for the dedicated status endpoint
    const statusChange = changes.status
    delete changes.status

    const onCloseFn = () => onClose()

    if (Object.keys(changes).length > 0 && statusChange) {
      // Both non-status changes and status change: update first, then status
      updateMutation.mutate(changes, {
        onSuccess: () => {
          statusMutation.mutate(statusChange, {
            onSuccess: onCloseFn,
          })
        },
        onError: onCloseFn,
      })
    } else if (Object.keys(changes).length > 0) {
      updateMutation.mutate(changes, {
        onSuccess: onCloseFn,
      })
    } else if (statusChange) {
      statusMutation.mutate(statusChange, {
        onSuccess: onCloseFn,
      })
    } else {
      onClose()
    }
  }

  const handleCancel = () => {
    if (task) {
      setDraft({
        title: task.title,
        description: task.description ?? '',
        status: task.status,
        assigned_to: task.assigned_to ?? '',
        assignee_type: task.assignee_type ?? '',
      })
    }
    onClose()
  }

  // ── Assignee helpers ───────────────────────────────────────────────────────

  const assigneeOptions = [
    ...users.map((u) => ({ id: u.id, name: u.display_name || u.username, type: AssigneeType.Human })),
    ...sessions.map((s) => ({ id: s.id, name: s.id, type: AssigneeType.Session })),
  ]

  const getAssigneeName = (id: string): string => {
    const option = assigneeOptions.find((o) => o.id === id)
    return option?.name ?? id
  }

  const filteredOptions = searchTerm
    ? assigneeOptions.filter((o) => o.name.toLowerCase().includes(searchTerm.toLowerCase()))
    : assigneeOptions

  // ── Dependency helpers ─────────────────────────────────────────────────────

  const availableDepTasks = storyTasks.filter(
    (t) => t.id !== task.id && !blockers.some((b) => b.id === t.id)
  )

  const dependents = data?.dependents ?? []

  // ── Status label mapping ───────────────────────────────────────────────────

  const statusLabels: Record<string, string> = {
    new: 'New',
    ready: 'Ready',
    in_progress: 'In Progress',
    blocked: 'Blocked',
    done: 'Done',
    canceled: 'Canceled',
    archived: 'Archived',
  }

// ── RENDER ─────────────────────────────────────────────────────────────────

  return (
    <SlideInPanel>
      {/* Header */}
      <div className="sticky top-0 bg-white dark:bg-charcoal-dark border-b border-gray-200 dark:border-gray-border px-4 py-3 z-10">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="font-mono text-xs text-neutral-500 dark:text-amber-muted">
              {task.id}
            </span>
            <SharpTag
              label={taskTypeLabel(task.task_type)}
              variant={taskTypeVariant(task.task_type)}
            />
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
        <EditableTitle
          value={draft?.title ?? task.title}
          onSave={(title) => setDraft((prev) => (prev ? { ...prev, title } : null))}
        />
      </div>

      {/* Scrollable content area */}
      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-5">
        {/* 1. Description */}
        <div>
          <FieldLabel>Description</FieldLabel>
          <textarea
            value={draft?.description ?? task.description ?? ''}
            onChange={(e) =>
              setDraft((prev) => (prev ? { ...prev, description: e.target.value } : null))
            }
            rows={4}
            className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-3 font-mono text-sm text-neutral-800 dark:text-light-neutral resize-y"
            placeholder="Markdown description..."
          />
        </div>

        {/* 2. Task Type (read-only) */}
        <div>
          <FieldLabel>Task Type</FieldLabel>
          <SharpTag
            label={taskTypeLabel(task.task_type)}
            variant={taskTypeVariant(task.task_type)}
          />
        </div>

        {/* 3. Status */}
        <div>
          <FieldLabel margin="mb-2">Status</FieldLabel>
          <select
            value={draft?.status ?? task.status}
            onChange={(e) =>
              setDraft((prev) =>
                prev ? { ...prev, status: e.target.value as StatusType } : null
              )
            }
            className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral"
          >
            {STATUS_ORDER.map((s) => (
              <option key={s} value={s}>
                {statusLabels[s] ?? s}
              </option>
            ))}
          </select>
        </div>

        {/* 4. Instructions (collapsible / advanced) */}
        {task.instructions && (
          <details className="group">
            <summary className="cursor-pointer text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 transition-colors">
              Instructions (advanced)
            </summary>
            <pre className="mt-2 font-mono text-sm bg-charcoal-darkest p-3 rounded-none border border-gray-border text-neutral-700 dark:text-neutral-300 whitespace-pre-wrap break-words">
              {task.instructions}
            </pre>
          </details>
        )}

        {/* 5. Dependencies */}
        <div>
          {/* Depends On (predecessors) */}
          <div className="mb-4">
            <FieldLabel margin="mb-2">Depends On</FieldLabel>
            {blockers.length > 0 ? (
              <div className="space-y-1 mb-2">
                {blockers.map((dep) => (
                  <div
                    key={dep.id}
                    className="flex items-center justify-between px-3 py-1 border border-gray-200 dark:border-gray-border"
                  >
                    <div className="flex items-center gap-2">
                      <span className="status-dot status-dot-warning" />
                      <span className="font-mono text-xs text-neutral-800 dark:text-light-neutral">
                        [{dep.id}] {dep.title}
                      </span>
                    </div>
                    <button
                      onClick={() => removeDepMutation.mutate(dep.id)}
                      className="p-0.5 text-neutral-400 hover:text-red-500 transition-colors"
                      aria-label="Remove dependency"
                    >
                      <X size={12} />
                    </button>
                  </div>
                ))}
              </div>
            ) : (
              <div className="flex items-center gap-2 px-3 py-2 border border-gray-200 dark:border-gray-border mb-2">
                <span className="font-mono text-xs text-neutral-400 dark:text-neutral-500">
                  No dependencies
                </span>
              </div>
            )}

            {/* Add dependency dropdown */}
            <div className="relative">
              <button
                onClick={() => setShowDepDropdown((prev) => !prev)}
                disabled={availableDepTasks.length === 0}
                className="text-xs font-mono uppercase tracking-wider text-purple-active hover:text-purple-400 transition-colors disabled:opacity-40"
              >
                + Add
              </button>
              {showDepDropdown && availableDepTasks.length > 0 && (
                <div className="absolute z-20 left-0 mt-1 w-full border border-gray-200 dark:border-gray-border bg-white dark:bg-charcoal-dark max-h-48 overflow-y-auto shadow-lg">
                  {availableDepTasks.map((t) => (
                    <button
                      key={t.id}
                      onMouseDown={() => {
                        addDepMutation.mutate(t.id)
                        setShowDepDropdown(false)
                      }}
                      className="w-full text-left px-3 py-1.5 hover:bg-neutral-100 dark:hover:bg-neutral-800 flex items-center gap-2"
                    >
                      <span className="font-mono text-xs text-neutral-800 dark:text-light-neutral">
                        [{t.id}] {t.title}
                      </span>
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Depended On By (successors) */}
          <div>
            <FieldLabel margin="mb-2">Depended On By</FieldLabel>
              {dependents.length > 0 ? (
              <div className="space-y-1">
                {dependents.map((dep: Task) => (
                  <div
                    key={dep.id}
                    className="flex items-center gap-2 px-3 py-1 border border-gray-200 dark:border-gray-border"
                  >
                    <span className="status-dot status-dot-info" />
                    <span className="font-mono text-xs text-neutral-800 dark:text-light-neutral">
                      [{dep.id}] {dep.title}
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <div className="flex items-center gap-2 px-3 py-2 border border-gray-200 dark:border-gray-border">
                <span className="font-mono text-xs text-neutral-400 dark:text-neutral-500">
                  No dependents
                </span>
              </div>
            )}
          </div>
        </div>

        {/* 6. Assigned To */}
        <div>
          <FieldLabel>Assigned To</FieldLabel>
          <div className="relative">
            {draft?.assigned_to ? (
              <div className="flex items-center gap-2 mb-1">
                <span className="font-mono text-sm text-neutral-800 dark:text-light-neutral">
                  {getAssigneeName(draft.assigned_to)}
                </span>
                <SharpTag
                  label={draft.assignee_type === AssigneeType.Session ? 'AGENT' : 'USER'}
                  variant={draft.assignee_type === AssigneeType.Session ? 'amber' : 'primary'}
                />
                <button
                  onClick={() =>
                    setDraft((prev) =>
                      prev ? { ...prev, assigned_to: '', assignee_type: '' } : null
                    )
                  }
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
                            setDraft((prev) =>
                              prev
                                ? { ...prev, assigned_to: opt.id, assignee_type: opt.type }
                                : null
                            )
                            setSearchTerm('')
                            setShowDropdown(false)
                          }}
                          className="w-full text-left px-3 py-2 hover:bg-neutral-100 dark:hover:bg-neutral-800 flex items-center gap-2"
                        >
                          <span className="font-mono text-sm text-neutral-800 dark:text-light-neutral">
                            {opt.name}
                          </span>
                          <SharpTag
                            label={opt.type === AssigneeType.Session ? 'AGENT' : 'USER'}
                            variant={opt.type === AssigneeType.Session ? 'amber' : 'primary'}
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

        {/* 7. CommentThread (moved inside scrollable area) */}
        <CommentThread workItemId={taskId} workItemType="task" />

        {/* 8. Delete Task */}
        <div className="border-t border-gray-200 dark:border-gray-border pt-4">
          <button
            onClick={() => setShowDeleteConfirm(true)}
            disabled={deleteMutation.isPending}
            className="text-xs font-mono uppercase tracking-wider text-red-500 hover:text-red-400 disabled:opacity-40 transition-colors"
          >
            <Trash2 size={14} className="inline mr-1.5 -mt-0.5" />
            Delete Task
          </button>
        </div>
      </div>

      {/* Save/Cancel bar at bottom (always visible, NOT sticky) */}
      <div className="border-t border-gray-200 dark:border-gray-border px-4 py-3 flex items-center justify-end gap-2">
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
                Save &amp; Close
              </button>
            </div>
          )}
        </div>
      </div>

      <ConfirmModal
        open={showDeleteConfirm}
        title="Delete Task"
        message="Are you sure you want to delete this task? This action cannot be undone."
        onConfirm={() => {
          setShowDeleteConfirm(false)
          deleteMutation.mutate()
        }}
        onCancel={() => setShowDeleteConfirm(false)}
      />
    </SlideInPanel>
  )
}

export default memo(TaskDetail)
