import SharpTag from './SharpTag'
import type { StatusType } from '../types'
import { statusVariant } from '../utils/status'

interface StatusTransitionsProps {
  currentStatus: StatusType
  transitions: string[]
  onTransition: (status: string) => void
  isPending: boolean
}

export default function StatusTransitions({
  currentStatus,
  transitions,
  onTransition,
  isPending,
}: StatusTransitionsProps) {
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <SharpTag
        label={currentStatus.toUpperCase()}
        variant={statusVariant(currentStatus)}
      />
      <div className="flex gap-1 flex-wrap">
        {transitions.map((s) => (
          <button
            key={s}
            onClick={() => onTransition(s)}
            disabled={isPending}
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
  )
}
