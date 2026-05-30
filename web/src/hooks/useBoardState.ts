import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useBoard } from './useBoard'
import { getUsers, fetchSessions } from '../api/client'
import type { Story, Task, User, Session } from '../types'

export interface BoardDisplayState {
  isLoading: boolean
  error: Error | null
  displayStories: Story[]
  displayTaskOrder: Record<string, string[]>
  tasksByStoryAndStatus: Record<string, Record<string, Task[]>>
  allTasks: Task[]
  assigneeNameMap: Record<string, string>
}

export function useBoardState(): BoardDisplayState {
  const { data, isLoading, error } = useBoard()

  const { data: users = [] } = useQuery<User[]>({
    queryKey: ['users'],
    queryFn: getUsers,
  })

  const { data: sessions = [] } = useQuery<Session[]>({
    queryKey: ['sessions'],
    queryFn: fetchSessions,
  })

  const displayStories: Story[] = useMemo(
    () => (data?.stories ?? []).sort((a, b) => a.sort_order - b.sort_order),
    [data?.stories],
  )

  const displayTaskOrder = useMemo(() => {
    const order: Record<string, string[]> = {}
    if (data?.tasks_by_story_and_status) {
      for (const [storyId, statusMap] of Object.entries(data.tasks_by_story_and_status)) {
        for (const [status, tasks] of Object.entries(statusMap)) {
          const key = `${storyId}:${status}`
          order[key] = [...tasks].sort((a, b) => a.sort_order - b.sort_order).map(t => t.id)
        }
      }
    }
    return order
  }, [data?.tasks_by_story_and_status])

  const tasksByStoryAndStatus = useMemo(
    () => data?.tasks_by_story_and_status ?? {},
    [data?.tasks_by_story_and_status],
  )

  const allTasks: Task[] = useMemo(() => {
    const tasks: Task[] = []
    if (data?.tasks_by_status) {
      for (const statusList of Object.values(data.tasks_by_status)) {
        for (const task of statusList) {
          tasks.push(task)
        }
      }
    }
    return tasks
  }, [data?.tasks_by_status])

  const assigneeNameMap: Record<string, string> = useMemo(() => {
    const map: Record<string, string> = {}
    for (const u of users) {
      map[u.id] = u.display_name || u.username
    }
    for (const s of sessions) {
      map[s.id] = s.id
    }
    return map
  }, [users, sessions])

  return {
    isLoading,
    error: error ?? null,
    displayStories,
    displayTaskOrder,
    tasksByStoryAndStatus,
    allTasks,
    assigneeNameMap,
  }
}
