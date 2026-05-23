import type { ReactNode } from 'react'

interface SlideInPanelProps {
  children: ReactNode
}

export default function SlideInPanel({ children }: SlideInPanelProps) {
  return (
    <div className="fixed right-0 top-[52px] bottom-0 w-[480px] bg-white dark:bg-charcoal-dark border-l border-gray-200 dark:border-gray-border rounded-none shadow-none overflow-y-auto z-40">
      {children}
    </div>
  )
}

export function PanelLoading({ message = 'Loading...' }: { message?: string }) {
  return (
    <SlideInPanel>
      <div className="flex items-center justify-center h-64">
        <span className="font-mono text-sm text-neutral-500 dark:text-amber-muted">
          {message}
        </span>
      </div>
    </SlideInPanel>
  )
}

export function PanelNotFound({ message = 'Not found' }: { message?: string }) {
  return (
    <SlideInPanel>
      <div className="flex items-center justify-center h-64">
        <span className="font-mono text-sm text-red-500">{message}</span>
      </div>
    </SlideInPanel>
  )
}
