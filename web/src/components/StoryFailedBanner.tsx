import { memo, useEffect, useState } from 'react'
import { AlertTriangle, X } from 'lucide-react'

export interface FailedStoryInfo {
  storyId: string
  title: string
  failureCount: number
}

interface StoryFailedBannerProps {
  story: FailedStoryInfo
  onDismiss: (storyId: string) => void
  /** Duration in ms before auto-dismiss. Default 10000. */
  autoDismissMs?: number
}

function StoryFailedBanner({ story, onDismiss, autoDismissMs = 10000 }: StoryFailedBannerProps) {
  const [visible, setVisible] = useState(true)

  useEffect(() => {
    const timer = setTimeout(() => {
      setVisible(false)
      onDismiss(story.storyId)
    }, autoDismissMs)
    return () => clearTimeout(timer)
  }, [story.storyId, onDismiss, autoDismissMs])

  if (!visible) return null

  return (
    <div className="flex items-start gap-3 border-l-4 border-red-500 bg-red-50 dark:bg-red-900/20 px-4 py-3 shadow-sm">
      <AlertTriangle size={18} className="mt-0.5 shrink-0 text-red-500" />
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold text-red-800 dark:text-red-200">
          Story Failed: {story.title}
        </p>
        <p className="text-xs text-red-600 dark:text-red-300 mt-0.5 font-mono">
          {story.storyId} &mdash; circuit breaker tripped ({story.failureCount} failure{story.failureCount !== 1 ? 's' : ''})
        </p>
      </div>
      <button
        onClick={() => {
          setVisible(false)
          onDismiss(story.storyId)
        }}
        className="shrink-0 p-1 text-red-400 hover:text-red-600 dark:hover:text-red-200 transition-colors"
        aria-label="Dismiss"
      >
        <X size={16} />
      </button>
    </div>
  )
}

export default memo(StoryFailedBanner)
