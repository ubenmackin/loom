import type { ReactNode } from 'react'

interface AsyncBoundaryProps {
  isLoading: boolean
  error: Error | null
  onRetry: () => void
  isEmpty: boolean
  emptyMessage?: string
  children: ReactNode
}

export default function AsyncBoundary({
  isLoading,
  error,
  onRetry,
  isEmpty,
  emptyMessage = 'No data',
  children,
}: AsyncBoundaryProps) {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <span className="font-mono text-sm text-neutral-500 dark:text-amber-muted">
          Loading...
        </span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-64 gap-2">
        <span className="font-mono text-sm text-red-500">
          Error: {error.message}
        </span>
        <button onClick={() => onRetry()} className="glow-button text-xs">
          Retry
        </button>
      </div>
    )
  }

  if (isEmpty) {
    return (
      <div className="flex items-center justify-center py-16">
        <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-600 uppercase tracking-widest">
          {emptyMessage}
        </span>
      </div>
    )
  }

  return <>{children}</>
}