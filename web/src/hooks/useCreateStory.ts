import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createStory } from '../api/client'

export function useCreateStory() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createStory,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['board'] })
    },
  })
}
