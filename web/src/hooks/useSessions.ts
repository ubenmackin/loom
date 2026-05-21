import { useQuery } from '@tanstack/react-query'
import { fetchSessions } from '../api/client'
import type { Session } from '../types'

export function useSessions() {
  return useQuery<Session[]>({
    queryKey: ['sessions'],
    queryFn: fetchSessions,
    staleTime: 10_000,
  })
}
