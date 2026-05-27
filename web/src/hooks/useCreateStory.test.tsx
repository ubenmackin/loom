import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useCreateStory } from './useCreateStory'

vi.mock('../api/client', () => ({
  createStory: vi.fn(),
}))

import { createStory } from '../api/client'
import type { Story } from '../types'

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  }
}

const mockStory: Story = {
  id: 'story-new',
  title: 'Test Story',
  status: 'new',
  requires_build: false,
  requires_review: false,
  sort_order: 1,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

describe('useCreateStory', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    vi.clearAllMocks()
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
      },
    })
  })

  it('calls createStory with the passed data on mutate', async () => {
    vi.mocked(createStory).mockResolvedValueOnce(mockStory)

    const { result } = renderHook(() => useCreateStory(), {
      wrapper: createWrapper(queryClient),
    })

    const data: Partial<Story> = { title: 'Test Story', status: 'new' }
    result.current.mutate(data)

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(vi.mocked(createStory)).toHaveBeenCalledWith(data, expect.any(Object))
  })

  it('invalidates the board query on successful mutation', async () => {
    vi.mocked(createStory).mockResolvedValueOnce(mockStory)

    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

    const { result } = renderHook(() => useCreateStory(), {
      wrapper: createWrapper(queryClient),
    })

    const data: Partial<Story> = { title: 'Test Story', status: 'new' }
    result.current.mutate(data)

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['board'] })
  })
})
