import { useQuery } from '@tanstack/react-query'
import { fetchActivityLog } from '../api/client'
import type { ActivityLogEntry } from '../types'

export function useActivity(limit = 100) {
  return useQuery<ActivityLogEntry[]>({
    queryKey: ['activity', limit],
    queryFn: () => fetchActivityLog(limit),
    staleTime: 10_000,
  })
}
