import { useDroppable } from '@dnd-kit/core'
import type { StatusType } from '../types'

/** Droppable wrapper for a single (story × status) cell */
export function CellDropZone({
  id,
  storyId,
  status,
  children,
}: {
  id: string
  storyId: string
  status: StatusType
  children: React.ReactNode
}) {
  const { setNodeRef } = useDroppable({
    id,
    data: {
      type: 'cell',
      storyId,
      status,
    },
  })

  return (
    <div ref={setNodeRef} className="min-h-[40px]">
      {children}
    </div>
  )
}
