import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Pencil, Check, X, Send } from 'lucide-react'
import {
  fetchComments,
  addComment,
  updateComment,
  deleteComment,
  fetchActivity,
} from '../api/client'
import type { Comment, ActivityLogEntry, WorkItemTypeType } from '../types'

interface CommentThreadProps {
  workItemId: string
  workItemType: WorkItemTypeType
}

interface TimelineEntry {
  id: string
  type: 'comment' | 'activity'
  created_at: string
  data: Comment | ActivityLogEntry
}

export default function CommentThread({ workItemId, workItemType }: CommentThreadProps) {
  const queryClient = useQueryClient()
  const [newBody, setNewBody] = useState('')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editBody, setEditBody] = useState('')

  const { data: comments = [] } = useQuery<Comment[]>({
    queryKey: ['comments', workItemId, workItemType],
    queryFn: () => fetchComments(workItemId, workItemType),
    enabled: !!workItemId,
  })

  const { data: activities = [] } = useQuery<ActivityLogEntry[]>({
    queryKey: ['activity', workItemId, workItemType],
    queryFn: () => fetchActivity(workItemId),
    enabled: !!workItemId,
  })

  // Merge and sort chronologically
  const timeline: TimelineEntry[] = (() => {
    const items: (TimelineEntry & { _ts: number })[] = [
      ...comments.map((c) => ({ id: c.id, type: 'comment' as const, created_at: c.created_at, data: c, _ts: new Date(c.created_at).getTime() })),
      ...activities.map((a) => ({ id: a.id, type: 'activity' as const, created_at: a.created_at, data: a, _ts: new Date(a.created_at).getTime() })),
    ]
    items.sort((a, b) => a._ts - b._ts)
    return items.map(({ _ts, ...rest }) => rest)
  })()

  const addMutation = useMutation({
    mutationFn: (body: string) =>
      addComment(workItemId, workItemType, {
        body,
        author_id: 'current-user',
        author_type: 'human',
      }),
    onMutate: async (body) => {
      await queryClient.cancelQueries({ queryKey: ['comments', workItemId, workItemType] })
      const previous = queryClient.getQueryData<Comment[]>(['comments', workItemId, workItemType])
      const optimistic: Comment = {
        id: `temp-${Date.now()}`,
        work_item_id: workItemId,
        work_item_type: workItemType,
        author_id: 'current-user',
        author_type: 'human',
        body,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      }
      if (previous) {
        queryClient.setQueryData(['comments', workItemId, workItemType], [...previous, optimistic])
      }
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(['comments', workItemId, workItemType], context.previous)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['comments', workItemId, workItemType] })
    },
  })

  const editMutation = useMutation({
    mutationFn: ({ id, body }: { id: string; body: string }) =>
      updateComment(id, { body }),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['comments', workItemId, workItemType] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteComment(id),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['comments', workItemId, workItemType] })
    },
  })

  const handleAddComment = () => {
    if (newBody.trim()) {
      addMutation.mutate(newBody.trim())
      setNewBody('')
    }
  }

  const handleEditSave = (id: string) => {
    if (editBody.trim()) {
      editMutation.mutate({ id, body: editBody.trim() })
    }
    setEditingId(null)
    setEditBody('')
  }

  const formatTimestamp = (iso: string) => {
    const d = new Date(iso)
    return d.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  return (
    <div className="mt-5 pt-4 border-t border-gray-200 dark:border-gray-border">
      <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-3">
        Activity & Comments
      </label>

      {/* Timeline */}
      <div className="space-y-0 max-h-[320px] overflow-y-auto">
        {timeline.length === 0 && (
          <div className="py-2">
            <span className="font-mono text-xs text-neutral-400 dark:text-neutral-500">
              No activity yet
            </span>
          </div>
        )}

        {timeline.map((entry) => {
          if (entry.type === 'activity') {
            const activity = entry.data as ActivityLogEntry
            return (
              <div
                key={entry.id}
                className="font-mono text-xs text-gray-500 dark:text-gray-400 py-1"
              >
                <span className="text-neutral-400 dark:text-neutral-500">
                  {formatTimestamp(activity.created_at)}
                </span>{' '}
                — {activity.action}
                {activity.details && (
                  <span className="text-neutral-500 dark:text-neutral-600">
                    {' '}
                    — {activity.details}
                  </span>
                )}
              </div>
            )
          }

          const comment = entry.data as Comment
          const isAgent = comment.author_type === 'session'
          const isEditing = editingId === comment.id
          const isOptimistic = comment.id.startsWith('temp-')

          return (
            <div
              key={entry.id}
              className={`border-l-2 ${
                isAgent
                  ? 'border-purple-active'
                  : 'border-amber-primary'
              } border-b border-gray-200 dark:border-gray-border py-2 px-3 ${
                isOptimistic ? 'opacity-60' : ''
              }`}
            >
              <div className="flex items-center justify-between mb-1">
                <div className="flex items-center gap-2">
                  <span className="font-mono text-[10px] text-neutral-500 dark:text-neutral-400">
                    {comment.author_id}
                  </span>
                  {isAgent && (
                    <span className="status-dot status-dot-info status-dot-pulse" />
                  )}
                  <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-600">
                    {formatTimestamp(comment.created_at)}
                  </span>
                </div>
                {comment.author_id === 'current-user' && !isOptimistic && (
                  <div className="flex items-center gap-1">
                    {!isEditing && (
                      <button
                        onClick={() => {
                          setEditingId(comment.id)
                          setEditBody(comment.body ?? '')
                        }}
                        className="p-0.5 text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-200 transition-colors"
                        aria-label="Edit comment"
                      >
                        <Pencil size={12} />
                      </button>
                    )}
                    <button
                      onClick={() => deleteMutation.mutate(comment.id)}
                      className="p-0.5 text-neutral-400 hover:text-red-500 transition-colors"
                      aria-label="Delete comment"
                    >
                      <X size={12} />
                    </button>
                  </div>
                )}
              </div>

              {isEditing ? (
                <div className="space-y-1">
                  <textarea
                    value={editBody}
                    onChange={(e) => setEditBody(e.target.value)}
                    rows={3}
                    className="w-full rounded-none border border-gray-200 dark:border-gray-border bg-charcoal-darkest p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral resize-y"
                    autoFocus
                  />
                  <div className="flex gap-1">
                    <button
                      onClick={() => handleEditSave(comment.id)}
                      className="glow-button px-2 py-1 text-[10px]"
                    >
                      <Check size={12} />
                    </button>
                    <button
                      onClick={() => {
                        setEditingId(null)
                        setEditBody('')
                      }}
                      className="px-2 py-1 rounded-none border border-gray-300 dark:border-gray-border text-[10px] text-neutral-500 dark:text-neutral-400 hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              ) : (
                <pre className="font-mono text-sm text-neutral-700 dark:text-neutral-300 whitespace-pre-wrap break-words">
                  {comment.body}
                </pre>
              )}
            </div>
          )
        })}
      </div>

      {/* New comment form */}
      <div className="mt-3 pt-3 border-t border-gray-200 dark:border-gray-border">
        <div className="flex gap-2">
          <textarea
            value={newBody}
            onChange={(e) => setNewBody(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                handleAddComment()
              }
            }}
            rows={2}
            placeholder="Add a comment... (Cmd+Enter to send)"
            className="flex-1 rounded-none border border-gray-200 dark:border-gray-border bg-transparent p-2 font-mono text-sm text-neutral-800 dark:text-light-neutral resize-none"
          />
          <button
            onClick={handleAddComment}
            disabled={!newBody.trim() || addMutation.isPending}
            className="glow-button self-end px-3 py-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Send size={14} />
          </button>
        </div>
      </div>
    </div>
  )
}
