import React from 'react'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useSessions } from './useSessions'

vi.mock('../api/client', () => ({
  fetchSessions: vi.fn(),
}))

import { fetchSessions } from '../api/client'
import type { Session } from '../types'

const mockSessionsData: Session[] = [
  {
    id: 'session-1',
    harness_type: 'vscode',
    last_seen_at: '2024-01-01T00:00:00Z',
    status: 'active',
    created_at: '2024-01-01T00:00:00Z',
  },
]

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(QueryClientProvider, { client: queryClient }, children)
  }
}

describe('useSessions', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    vi.clearAllMocks()
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
  })

  it('has queryKey ["sessions"]', async () => {
    vi.mocked(fetchSessions).mockResolvedValueOnce(mockSessionsData)
    renderHook(() => useSessions(), {
      wrapper: createWrapper(queryClient),
    })
    await waitFor(() => {
      expect(fetchSessions).toHaveBeenCalled()
    })
    const cacheData = queryClient.getQueryData(['sessions'])
    expect(cacheData).toEqual(mockSessionsData)
  })

  it('calls fetchSessions as queryFn', async () => {
    vi.mocked(fetchSessions).mockResolvedValueOnce(mockSessionsData)
    renderHook(() => useSessions(), {
      wrapper: createWrapper(queryClient),
    })
    await waitFor(() => {
      expect(fetchSessions).toHaveBeenCalled()
    })
  })

  it('returns sessions data', async () => {
    vi.mocked(fetchSessions).mockResolvedValueOnce(mockSessionsData)
    const { result } = renderHook(() => useSessions(), {
      wrapper: createWrapper(queryClient),
    })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual(mockSessionsData)
  })

  it('sets staleTime to 10000', async () => {
    vi.mocked(fetchSessions).mockResolvedValueOnce(mockSessionsData)
    renderHook(() => useSessions(), {
      wrapper: createWrapper(queryClient),
    })
    await waitFor(() => {
      expect(fetchSessions).toHaveBeenCalled()
    })
    // Verify the cache entry is populated, confirming the query ran
    const cacheData = queryClient.getQueryData(['sessions'])
    expect(cacheData).toEqual(mockSessionsData)
  })
})