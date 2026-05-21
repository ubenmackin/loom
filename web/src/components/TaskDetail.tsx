import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { X, Check, Pencil, Play, AlertCircle } from 'lucide-react'
import SharpTag from './SharpTag'
import CommentThread from './CommentThread'
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
} from '../api/client'
import type { Task } from '../types'
import { useSessionStore } from '../stores/session'
import { statusVariant } from '../utils/statusVariant'
import { taskTypeLabel } from '../utils/taskTypeLabel'
import { taskTypeVariant } from '../utils/taskTypeVariant'
import { STATUS_ORDER, VALID_TRANSITIONS } from '../utils/statusConstants'

interface TaskDetailProps {
  taskId: string | null
  onClose: () => void
}

export default function TaskDetail({ taskId, onClose }: TaskDetailProps) {
  const queryClient = useQueryClient()
  const sessionId = useSessionStore((s) => s.sessionId)
  const [editingTitle, setEditingTitle] = useState(false)
  const [titleValue, setTitleValue] = useState('')
  const [descValue, setDescValue] = useState('')
  const [depInput, setDepInput] = useState('')

  const { data: task, isLoading } = useQuery<Task>({
    queryKey: ['task', taskId],
    queryFn: () => fetchTask(taskId!),
    enabled: !!taskId,
  })

  const { data: blockers = [] } = useQuery({
    queryKey: ['blockers', taskId],
    queryFn: () => fetchBlockers(taskId!),
    enabled: !!taskId,
  })

  const updateMutation = useMutation({
    mutationFn: (data: Partial<Task>) => updateTask(taskId!, data),
    onMutate: async (data) => {
      await queryClient.cancelQueries({ queryKey: ['task', taskId] })
      const previous = queryClient.getQueryData<Task>(['task', taskId])
      if (previous) {
        queryClient.setQueryData(['task', taskId], { ...previous, ...data })
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
    onMutate: async (status) => {
      await queryClient.cancelQueries({ queryKey: ['task', taskId] })
      const previous = queryClient.getQueryData<Task>(['task', taskId])
      if (previous) {
        queryClient.setQueryData(['task', taskId], { ...previous, status })
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

  if (!taskId) return null

  if (isLoading) {
    return (
      <div className="fixed right-0 top-[52px] bottom-0 w-[480px] bg-white dark:bg-charcoal-dark border-l border-gray-200 dark:border-gray-border rounded-none shadow-none overflow-y-auto z-40">
        <div className="flex items-center justify-center h-64">
          <span className="font-mono text-sm text-neutral-500 dark:text-amber-muted">
            Loading task...
          </span>
        </div>
      </div>
    )
  }

  if (!task) {
    return (
      <div className="fixed right-0 top-[52px] bottom-0 w-[480px] bg-white dark:bg-charcoal-dark border-l border-gray-200 dark:border-gray-border rounded-none shadow-none overflow-y-auto z-40">
        <div className="flex items-center justify-center h-64">
          <span className="font-mono text-sm text-red-500">Task not found</span>
        </div>
      </div>
    )
  }

  const transitions = VALID_TRANSITIONS[task.status] ?? []

  const handleTitleSave = () => {
    if (titleValue.trim() && titleValue !== task.title) {
      updateMutation.mutate({ title: titleValue.trim() })
    }
    setEditingTitle(false)
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
    <div className="fixed right-0 top-[52px] bottom-0 w-[480px] bg-white dark:bg-charcoal-dark border-l border-gray-200 dark:border-gray-border rounded-none shadow-none overflow-y-auto z-40">
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
        {editingTitle ? (
          <div className="mt-2 flex gap-2">
            <input
              type="text"
              value={titleValue}
              onChange={(e) => setTitleValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleTitleSave()
                if (e.key === 'Escape') {
                  setEditingTitle(false)
                  setTitleValue(task.title)
                }
              }}
              className="flex-1 rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 text-sm text-neutral-800 dark:text-light-neutral font-mono"
              autoFocus
            />
            <button
              onClick={handleTitleSave}
              className="glow-button px-3 py-2"
            >
              <Check size={14} />
            </button>
          </div>
        ) : (
          <button
            onClick={() => {
              setEditingTitle(true)
              setTitleValue(task.title)
            }}
            className="mt-1 text-left text-sm font-bold text-neutral-800 dark:text-light-neutral hover:text-loom-600 dark:hover:text-purple-active transition-colors w-full"
          >
            {task.title}
          </button>
        )}
      </div>

      {/* Fields */}
      <div className="px-4 py-4 space-y-5">
        {/* Description */}
        <div>
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
            Description
          </label>
          <textarea
            value={task.description ?? ''}
            onChange={(e) => setDescValue(e.target.value)}
            onBlur={handleDescSave}
            rows={4}
            className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-3 font-mono text-sm text-neutral-800 dark:text-light-neutral resize-y"
            placeholder="Markdown description..."
          />
        </div>

        {/* Task type */}
        <div>
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
            Task Type
          </label>
          <SharpTag
            label={taskTypeLabel(task.task_type)}
            variant={taskTypeVariant(task.task_type)}
          />
        </div>

        {/* Priority */}
        <div>
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
            Priority
          </label>
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
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
            Estimate
          </label>
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
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-2">
            Status
          </label>
          <div className="flex items-center gap-2 flex-wrap">
            <SharpTag
              label={task.status.toUpperCase()}
              variant={statusVariant(task.status)}
            />
            <div className="flex gap-1 flex-wrap">
              {transitions.map((s) => (
                <button
                  key={s}
                  onClick={() => statusMutation.mutate(s)}
                  disabled={statusMutation.isPending}
                  aria-label={`Transition to ${s.toUpperCase()} status`}
                  className={`px-2 py-1 rounded-none border text-[10px] uppercase tracking-wider font-mono transition-colors ${
                    s === 'done'
                      ? 'glow-button px-2 py-1 text-[10px]'
                      : 'border-gray-300 dark:border-gray-border text-neutral-500 dark:text-neutral-400 hover:bg-neutral-100 dark:hover:bg-neutral-800'
                  }`}
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Context JSON */}
        <div>
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
            Context JSON
          </label>
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
            <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
              Instructions
            </label>
            <pre className="font-mono text-sm bg-charcoal-darkest p-3 rounded-none border border-gray-border text-neutral-700 dark:text-neutral-300 whitespace-pre-wrap break-words">
              {task.instructions}
            </pre>
          </div>
        )}

        {/* Dependencies */}
        <div>
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-2">
            Dependencies ({blockers.length})
          </label>

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
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
            Assigned To
          </label>
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
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-2">
            Actions
          </label>
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
    </div>
  )
}
