import { useQuery } from '@tanstack/react-query'
import { fetchBoard } from '../api/client'
import type { BoardState } from '../types'

export function useBoard() {
  return useQuery<BoardState>({
    queryKey: ['board'],
    queryFn: fetchBoard,
  })
}
