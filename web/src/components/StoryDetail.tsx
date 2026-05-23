import { memo, useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { X, AlertCircle } from 'lucide-react'
import SharpTag from './SharpTag'
import SlideInPanel, { PanelLoading, PanelNotFound } from './SlideInPanel'
import EditableTitle from './EditableTitle'
import FieldLabel from './FieldLabel'
import StatusTransitions from './StatusTransitions'
import {
  fetchStory,
  updateStory,
  updateStoryStatus,
} from '../api/client'
import type { Story, StoryWithTasks } from '../types'
import { statusVariant, VALID_TRANSITIONS } from '../utils/status'

interface StoryDetailProps {
  storyId: string | null
  onClose: () => void
  onOpenTask?: (taskId: string) => void
}

function StoryDetail({ storyId, onClose, onOpenTask }: StoryDetailProps) {
  const queryClient = useQueryClient()
  const [descValue, setDescValue] = useState('')

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

  const statusMutation = useMutation({
    mutationFn: (status: string) => updateStoryStatus(storyId!, status),
    onMutate: async (status) => {
      await queryClient.cancelQueries({ queryKey: ['story', storyId] })
      const previous = queryClient.getQueryData<StoryWithTasks>(['story', storyId])
      if (previous) {
        queryClient.setQueryData(['story', storyId], {
          ...previous,
          story: { ...previous.story, status },
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

  // Sync descValue from story.description whenever story changes
  useEffect(() => {
    if (story) {
      setDescValue(story.description ?? '')
    }
  }, [story])

  if (!storyId) return null

  if (isLoading) {
    return <PanelLoading message="Loading story..." />
  }

  if (!story) {
    return <PanelNotFound message="Story not found" />
  }

  const transitions = VALID_TRANSITIONS[story.status] ?? []

  const handleTitleSave = (title: string) => {
    updateMutation.mutate({ title })
  }

  const handleDescSave = () => {
    if (descValue !== story.description) {
      updateMutation.mutate({ description: descValue })
    }
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
        <EditableTitle value={story.title} onSave={handleTitleSave} />
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

        {/* Priority */}
        <div>
          <FieldLabel>Priority</FieldLabel>
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
          <FieldLabel margin="mb-2">Status</FieldLabel>
          <StatusTransitions
            currentStatus={story.status}
            transitions={transitions}
            onTransition={(s) => statusMutation.mutate(s)}
            isPending={statusMutation.isPending}
          />
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
          <FieldLabel>Assigned To</FieldLabel>
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
        </div>
      </div>
    </SlideInPanel>
  )
}

export default memo(StoryDetail)
