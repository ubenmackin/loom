import { useQuery } from '@tanstack/react-query'
import { fetchBoard } from '../api/client'
import { useProjectFilterStore } from '../stores/project'
import type { BoardState } from '../types'

export function useBoard() {
  const selectedProjectId = useProjectFilterStore((s) => s.selectedProjectId)

  return useQuery<BoardState>({
    queryKey: ['board', selectedProjectId],
    queryFn: () => fetchBoard(selectedProjectId ?? undefined),
  })
}