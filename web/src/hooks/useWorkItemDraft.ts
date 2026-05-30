import { useState, useEffect } from 'react'

/**
 * Generic hook for managing a draft/editing state for work items (Story, Task).
 *
 * Provides:
 * - `draft` — mutable copy of the server data
 * - `setDraft` — updater for the draft
 * - `isDirty` — whether the draft differs from the original server data
 * - `computeChanges` — returns only the changed fields from the draft
 * - `reset` — resets the draft to match the server data
 *
 * @param serverData — the latest server-fetched object (or null)
 * @param keys — the field names to track for dirty/compute-changes
 */
export function useWorkItemDraft<T extends object>(
  serverData: T | null | undefined,
  keys: (keyof T)[],
) {
  const [draft, setDraft] = useState<T | null>(null)

  // Sync draft from server data whenever server data changes
  useEffect(() => {
    if (serverData) {
      setDraft({ ...serverData })
    }
  }, [serverData])

  const isDirty = Boolean(
    draft &&
      serverData &&
      keys.some((key) => draft[key] !== serverData[key]),
  )

  const computeChanges = (): Partial<T> | null => {
    if (!draft || !serverData || !isDirty) return null
    const changes = {} as Partial<T>
    for (const key of keys) {
      if (draft[key] !== serverData[key]) {
        changes[key] = draft[key]
      }
    }
    return changes
  }

  const reset = () => {
    if (serverData) {
      setDraft({ ...serverData })
    }
  }

  return { draft, setDraft, isDirty, computeChanges, reset }
}