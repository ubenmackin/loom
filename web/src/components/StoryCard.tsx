import { memo } from 'react'
import { CheckSquare } from 'lucide-react'
import SharpTag from './SharpTag'
import type { Story } from '../types'
import { statusVariant } from '../utils/status'

interface StoryCardProps {
  story: Story
  onClick?: () => void
  assigneeName?: string
}

function StoryCard({ story, onClick, assigneeName }: StoryCardProps) {
  return (
    <div
      className="border border-gray-200 dark:border-gray-border p-3 rounded-none shadow-none bg-white dark:bg-charcoal-dark cursor-pointer"
      onClick={onClick}
    >
      {/* Title row */}
      <div className="flex items-start justify-between gap-2">
        <span className="text-left text-sm font-bold text-neutral-800 dark:text-light-neutral leading-tight">
          {story.title}
        </span>
      </div>

      {/* Tags row */}
      <div className="flex items-center gap-1.5 mt-2 flex-wrap">
        <SharpTag label="STORY" variant="primary" />
        <SharpTag label={story.status.toUpperCase()} variant={statusVariant(story.status)} />
        {story.requires_build && (
          <span className="text-neutral-400 dark:text-neutral-500 hover:text-amber-primary dark:hover:text-amber-primary transition-colors">
            <CheckSquare size={12} />
          </span>
        )}
        {story.requires_review && (
          <span className="text-neutral-400 dark:text-neutral-500 hover:text-purple-active dark:hover:text-purple-active transition-colors">
            <CheckSquare size={12} />
          </span>
        )}
      </div>

      {/* Assigned agent */}
      {story.assigned_to && (
        <div className="mt-1.5 font-mono text-xs dark:text-amber-primary text-neutral-500">
          {assigneeName || story.assigned_to}
        </div>
      )}

      {/* Stale indicator */}
      {story.updated_at && isStale(story.updated_at) && (
        <div className="mt-1 flex items-center gap-1">
          <span className="status-dot status-dot-warning status-dot-pulse" />
          <span className="font-mono text-[10px] text-amber-500">stale</span>
        </div>
      )}
    </div>
  )
}

function isStale(updatedAt: string): boolean {
  const hours = (Date.now() - new Date(updatedAt).getTime()) / (1000 * 60 * 60)
  return hours > 2
}

export default memo(StoryCard)
