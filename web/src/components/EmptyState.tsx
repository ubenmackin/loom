interface EmptyStateProps {
  show: boolean
}

export default function EmptyState({ show }: EmptyStateProps) {
  if (!show) return null
  return (
    <div className="flex items-center justify-center py-8">
      <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-600 uppercase tracking-widest">
        Empty
      </span>
    </div>
  )
}