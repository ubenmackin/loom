import { memo, useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { X, Check, Play, AlertCircle } from 'lucide-react'
import SharpTag from './SharpTag'
import CommentThread from './CommentThread'
import SlideInPanel, { PanelLoading, PanelNotFound } from './SlideInPanel'
import EditableTitle from './EditableTitle'
import FieldLabel from './FieldLabel'
import StatusTransitions from './StatusTransitions'
import {
  fetchTask,
  updateTask,
  updateTaskStatus,
  fetchBlockers,
  addDependency,
  removeDependency,
  startWork,
  completeWork,
  blockWork,
  createTask,
  TaskDetailResponse,
} from '../api/client'
import type { Task, TaskTypeType } from '../types'
import { TaskType } from '../types'
import { useSessionStore } from '../stores/session'
import { statusVariant, VALID_TRANSITIONS } from '../utils/status'
import { taskTypeLabel, taskTypeVariant } from '../utils/taskType'

interface TaskDetailProps {
  taskId: string | null
  onClose: () => void
}

function TaskDetail({ taskId, onClose }: TaskDetailProps) {
  const queryClient = useQueryClient()
  const sessionId = useSessionStore((s) => s.sessionId)
  const [descValue, setDescValue] = useState('')
  const [depInput, setDepInput] = useState('')
  const [createTitle, setCreateTitle] = useState('')
  const [createType, setCreateType] = useState<TaskTypeType>(TaskType.Code)

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

  useEffect(() => {
    if (!isCreateMode) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isCreateMode, onClose])

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

  const startWorkMutation = useMutation({
    mutationFn: () => startWork(sessionId, taskId!),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['task', taskId] })
      queryClient.invalidateQueries({ queryKey: ['board'] })
    },
  })

  const completeWorkMutation = useMutation({
    mutationFn: () => completeWork(sessionId, taskId!, { task_id: taskId!, result: 'completed' }),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['task', taskId] })
      queryClient.invalidateQueries({ queryKey: ['board'] })
    },
  })

  const blockWorkMutation = useMutation({
    mutationFn: () => blockWork(sessionId, taskId!, { task_id: taskId!, reason: 'blocked' }),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['task', taskId] })
      queryClient.invalidateQueries({ queryKey: ['board'] })
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

  // Sync descValue from task.description whenever task changes
  useEffect(() => {
    if (task) {
      setDescValue(task.description ?? '')
    }
  }, [task])

  if (!taskId) return null

  // CREATE MODE
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

  const transitions = VALID_TRANSITIONS[task.status] ?? []

  const handleTitleSave = (title: string) => {
    updateMutation.mutate({ title })
  }

  const handleDescSave = () => {
    if (descValue !== task.description) {
      updateMutation.mutate({ description: descValue })
    }
  }

  const handleAddDep = () => {
    if (depInput.trim()) {
      addDepMutation.mutate(depInput.trim())
      setDepInput('')
    }
  }

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
        <EditableTitle value={task.title} onSave={handleTitleSave} />
      </div>

      {/* Fields */}
      <div className="px-4 py-4 space-y-5">
        {/* Description */}
        <div>
          <FieldLabel>Description</FieldLabel>
          <textarea
            value={descValue}
            onChange={(e) => setDescValue(e.target.value)}
            onBlur={handleDescSave}
            rows={4}
            className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-3 font-mono text-sm text-neutral-800 dark:text-light-neutral resize-y"
            placeholder="Markdown description..."
          />
        </div>

        {/* Task type */}
        <div>
          <FieldLabel>Task Type</FieldLabel>
          <SharpTag
            label={taskTypeLabel(task.task_type)}
            variant={taskTypeVariant(task.task_type)}
          />
        </div>

        {/* Priority */}
        <div>
          <FieldLabel>Priority</FieldLabel>
          <input
            type="number"
            value={task.priority}
            onChange={(e) =>
              updateMutation.mutate({ priority: parseInt(e.target.value, 10) || 0 })
            }
            className="w-20 rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral"
          />
        </div>

        {/* Estimate */}
        <div>
          <FieldLabel>Estimate</FieldLabel>
          <input
            type="number"
            value={task.estimate ?? ''}
            onChange={(e) =>
              updateMutation.mutate({
                estimate: e.target.value ? parseInt(e.target.value, 10) : undefined,
              })
            }
            placeholder="—"
            className="w-20 rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral"
          />
        </div>

        {/* Status */}
        <div>
          <FieldLabel margin="mb-2">Status</FieldLabel>
          <StatusTransitions
            currentStatus={task.status}
            transitions={transitions}
            onTransition={(s) => statusMutation.mutate(s)}
            isPending={statusMutation.isPending}
          />
        </div>

        {/* Context JSON */}
        <div>
          <FieldLabel>Context JSON</FieldLabel>
          <textarea
            value={task.context ?? ''}
            onChange={(e) => updateMutation.mutate({ context: e.target.value })}
            rows={3}
            className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-3 font-mono text-xs text-neutral-800 dark:text-light-neutral resize-y"
            placeholder='{"key": "value"}'
          />
        </div>

        {/* Instructions preview */}
        {task.instructions && (
          <div>
            <FieldLabel>Instructions</FieldLabel>
            <pre className="font-mono text-sm bg-charcoal-darkest p-3 rounded-none border border-gray-border text-neutral-700 dark:text-neutral-300 whitespace-pre-wrap break-words">
              {task.instructions}
            </pre>
          </div>
        )}

        {/* Dependencies */}
        <div>
          <FieldLabel margin="mb-2">
            Dependencies ({blockers.length})
          </FieldLabel>

          {/* Dependency list */}
          {blockers.length > 0 ? (
            <div className="space-y-1 mb-2">
              {blockers.map((dep) => (
                <div
                  key={dep.id}
                  className="flex items-center justify-between px-3 py-1 border border-gray-200 dark:border-gray-border"
                >
                  <div className="flex items-center gap-2">
                    <span className="status-dot status-dot-warning" />
                    <span className="mono-bracket">[{dep.id}]</span>
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

          {/* Add dependency */}
          <div className="flex gap-2">
            <input
              type="text"
              value={depInput}
              onChange={(e) => setDepInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleAddDep()
              }}
              placeholder="Task ID (e.g. TASK-001)"
              className="flex-1 rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-xs text-neutral-800 dark:text-light-neutral"
            />
            <button
              onClick={handleAddDep}
              disabled={!depInput.trim() || addDepMutation.isPending}
              className="px-3 py-2 rounded-none border border-gray-300 dark:border-gray-border text-xs text-neutral-500 dark:text-neutral-400 hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors disabled:opacity-50"
            >
              Add
            </button>
          </div>
        </div>

        {/* Assigned to */}
        <div>
          <FieldLabel>Assigned To</FieldLabel>
          {task.assigned_to ? (
            <div className="flex items-center gap-2">
              <span className="font-mono text-sm text-neutral-800 dark:text-light-neutral">
                {task.assigned_to}
              </span>
              <button
                onClick={() =>
                  updateMutation.mutate({ assigned_to: undefined, assignee_type: undefined })
                }
                className="text-[10px] uppercase tracking-wider text-red-500 hover:text-red-400 transition-colors"
              >
                Unassign
              </button>
            </div>
          ) : (
            <span className="font-mono text-xs text-neutral-400 dark:text-neutral-500">
              Unassigned
            </span>
          )}
        </div>

        {/* Action buttons */}
        <div className="pt-3 border-t border-gray-200 dark:border-gray-border">
          <FieldLabel margin="mb-2">Actions</FieldLabel>
          <div className="flex gap-2 flex-wrap">
            <button
              onClick={() => startWorkMutation.mutate()}
              disabled={startWorkMutation.isPending}
              className="glow-button flex items-center gap-1 disabled:opacity-50"
            >
              <Play size={12} />
              Start Work
            </button>
            <button
              onClick={() => completeWorkMutation.mutate()}
              disabled={completeWorkMutation.isPending}
              className="px-4 py-2 rounded-none border border-green-500 text-green-500 text-xs font-bold uppercase tracking-wider hover:bg-green-500/10 transition-colors flex items-center gap-1 disabled:opacity-50"
            >
              <Check size={12} />
              Complete
            </button>
            <button
              onClick={() => blockWorkMutation.mutate()}
              disabled={blockWorkMutation.isPending}
              className="px-4 py-2 rounded-none border border-red-500 text-red-500 text-xs font-bold uppercase tracking-wider hover:bg-red-500/10 transition-colors flex items-center gap-1 disabled:opacity-50"
            >
              <AlertCircle size={12} />
              Block
            </button>
          </div>
        </div>

        {/* Comment thread */}
        <CommentThread workItemId={taskId} workItemType="task" />
      </div>
    </SlideInPanel>
  )
}

export default memo(TaskDetail)
