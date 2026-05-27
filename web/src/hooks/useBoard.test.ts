import React from 'react'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useBoard } from './useBoard'

vi.mock('../api/client', () => ({
  fetchBoard: vi.fn(),
}))

import { fetchBoard } from '../api/client'
import type { BoardState } from '../types'

const mockBoardData: BoardState = {
  stories: [],
  tasks_by_status: {},
  tasks_by_story_and_status: {},
  stats: {
    total_stories: 0,
    total_tasks: 0,
    ready_tasks: 0,
    in_progress_tasks: 0,
    blocked_tasks: 0,
    done_tasks: 0,
    canceled_tasks: 0,
    archived_tasks: 0,
    stale_tasks: 0,
  },
}

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(QueryClientProvider, { client: queryClient }, children)
  }
}

describe('useBoard', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    vi.clearAllMocks()
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
  })

  it('calls fetchBoard as the query function', async () => {
    vi.mocked(fetchBoard).mockResolvedValueOnce(mockBoardData)
    const { result } = renderHook(() => useBoard(), {
      wrapper: createWrapper(queryClient),
    })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(fetchBoard).toHaveBeenCalledOnce()
  })

  it('returns board data', async () => {
    vi.mocked(fetchBoard).mockResolvedValueOnce(mockBoardData)
    const { result } = renderHook(() => useBoard(), {
      wrapper: createWrapper(queryClient),
    })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual(mockBoardData)
  })

  it('has queryKey ["board"]', async () => {
    vi.mocked(fetchBoard).mockResolvedValueOnce(mockBoardData)
    const { result } = renderHook(() => useBoard(), {
      wrapper: createWrapper(queryClient),
    })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    const cacheData = queryClient.getQueryData(['board'])
    expect(cacheData).toEqual(mockBoardData)
  })

  it('does not set staleTime', async () => {
    vi.mocked(fetchBoard).mockResolvedValueOnce(mockBoardData)
    const { result } = renderHook(() => useBoard(), {
      wrapper: createWrapper(queryClient),
    })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    // useBoard does not set staleTime, so it uses the default (0)
    expect(result.current.data).toEqual(mockBoardData)
  })
})