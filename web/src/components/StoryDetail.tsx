import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { X, Check, AlertCircle } from 'lucide-react'
import SharpTag from './SharpTag'
import TaskCard from './TaskCard'
import {
  fetchStory,
  updateStory,
  updateStoryStatus,
  fetchTasks,
} from '../api/client'
import type { Story, Task } from '../types'
import { statusVariant } from '../utils/statusVariant'
import { STATUS_ORDER, VALID_TRANSITIONS } from '../utils/statusConstants'

interface StoryDetailProps {
  storyId: string | null
  onClose: () => void
}

export default function StoryDetail({ storyId, onClose }: StoryDetailProps) {
  const queryClient = useQueryClient()
  const [editingTitle, setEditingTitle] = useState(false)
  const [titleValue, setTitleValue] = useState('')
  const [descValue, setDescValue] = useState('')

  const { data: story, isLoading } = useQuery<Story>({
    queryKey: ['story', storyId],
    queryFn: () => fetchStory(storyId!),
    enabled: !!storyId,
  })

  const { data: tasks } = useQuery<Task[]>({
    queryKey: ['tasks', storyId],
    queryFn: () => fetchTasks({ story_id: storyId! }),
    enabled: !!storyId,
  })

  const updateMutation = useMutation({
    mutationFn: (data: Partial<Story>) => updateStory(storyId!, data),
    onMutate: async (data) => {
      await queryClient.cancelQueries({ queryKey: ['story', storyId] })
      const previous = queryClient.getQueryData<Story>(['story', storyId])
      if (previous) {
        queryClient.setQueryData(['story', storyId], { ...previous, ...data })
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

  const statusMutation = useMutation({
    mutationFn: (status: string) => updateStoryStatus(storyId!, status),
    onMutate: async (status) => {
      await queryClient.cancelQueries({ queryKey: ['story', storyId] })
      const previous = queryClient.getQueryData<Story>(['story', storyId])
      if (previous) {
        queryClient.setQueryData(['story', storyId], { ...previous, status })
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

  if (!storyId) return null

  if (isLoading) {
    return (
      <div className="fixed right-0 top-[52px] bottom-0 w-[480px] bg-white dark:bg-charcoal-dark border-l border-gray-200 dark:border-gray-border rounded-none shadow-none overflow-y-auto z-40">
        <div className="flex items-center justify-center h-64">
          <span className="font-mono text-sm text-neutral-500 dark:text-amber-muted">
            Loading story...
          </span>
        </div>
      </div>
    )
  }

  if (!story) {
    return (
      <div className="fixed right-0 top-[52px] bottom-0 w-[480px] bg-white dark:bg-charcoal-dark border-l border-gray-200 dark:border-gray-border rounded-none shadow-none overflow-y-auto z-40">
        <div className="flex items-center justify-center h-64">
          <span className="font-mono text-sm text-red-500">Story not found</span>
        </div>
      </div>
    )
  }

  const transitions = VALID_TRANSITIONS[story.status] ?? []

  const handleTitleSave = () => {
    if (titleValue.trim() && titleValue !== story.title) {
      updateMutation.mutate({ title: titleValue.trim() })
    }
    setEditingTitle(false)
  }

  const handleDescSave = () => {
    if (descValue !== story.description) {
      updateMutation.mutate({ description: descValue })
    }
  }

  return (
    <div className="fixed right-0 top-[52px] bottom-0 w-[480px] bg-white dark:bg-charcoal-dark border-l border-gray-200 dark:border-gray-border rounded-none shadow-none overflow-y-auto z-40">
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
                  setTitleValue(story.title)
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
              setTitleValue(story.title)
            }}
            className="mt-1 text-left text-sm font-bold text-neutral-800 dark:text-light-neutral hover:text-loom-600 dark:hover:text-purple-active transition-colors w-full"
          >
            {story.title}
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
            value={story.description ?? ''}
            onChange={(e) => setDescValue(e.target.value)}
            onBlur={handleDescSave}
            rows={4}
            className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-3 font-mono text-sm text-neutral-800 dark:text-light-neutral resize-y"
            placeholder="Markdown description..."
          />
        </div>

        {/* Priority */}
        <div>
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
            Priority
          </label>
          <input
            type="number"
            value={story.priority}
            onChange={(e) =>
              updateMutation.mutate({ priority: parseInt(e.target.value, 10) || 0 })
            }
            className="w-20 rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral"
          />
        </div>

        {/* Status */}
        <div>
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-2">
            Status
          </label>
          <div className="flex items-center gap-2 flex-wrap">
            <SharpTag label={story.status.toUpperCase()} variant={statusVariant(story.status)} />
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

        {/* Build toggle */}
        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={story.requires_build}
            onChange={(e) => updateMutation.mutate({ requires_build: e.target.checked })}
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
            checked={story.requires_review}
            onChange={(e) => updateMutation.mutate({ requires_review: e.target.checked })}
            className="rounded-none accent-purple-active"
          />
          <SharpTag label="REVIEW" variant="success" />
          <span className="text-xs text-neutral-500 dark:text-neutral-400">
            Requires review
          </span>
        </div>

        {/* Assigned to */}
        <div>
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-1">
            Assigned To
          </label>
          {story.assigned_to ? (
            <div className="flex items-center gap-2">
              <span className="font-mono text-sm text-neutral-800 dark:text-light-neutral">
                {story.assigned_to}
              </span>
              <button
                onClick={() => updateMutation.mutate({ assigned_to: undefined, assignee_type: undefined })}
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

        {/* Child tasks */}
        <div>
          <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-2">
            Child Tasks ({tasks?.length ?? 0})
          </label>
          {tasks && tasks.length > 0 ? (
            <div className="space-y-2">
              {tasks.map((task) => (
                <button
                  key={task.id}
                  onClick={() => {
                    // TaskDetail would be opened via state management
                    const event = new CustomEvent('open-task-detail', { detail: { taskId: task.id } })
                    window.dispatchEvent(event)
                  }}
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
        </div>
      </div>
    </div>
  )
}
