import type { ReactNode } from 'react'

interface FieldLabelProps {
  children: ReactNode
  /** Margin-bottom class override. Defaults to 'mb-1'. */
  margin?: 'mb-1' | 'mb-2'
  /** Optional htmlFor to associate the label with an input id */
  htmlFor?: string
}

export default function FieldLabel({ children, margin = 'mb-1', htmlFor }: FieldLabelProps) {
  return (
    <label
      htmlFor={htmlFor}
      className={`text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block ${margin}`}
    >
      {children}
    </label>
  )
}
