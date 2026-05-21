import type { TaskTypeType } from '../types'

export function taskTypeLabel(type: TaskTypeType): string {
  switch (type) {
    case 'code':
      return 'CODE'
    case 'build':
      return 'BUILD'
    case 'review':
      return 'REVIEW'
    default: {
      const _exhaustive: never = type
      return String(_exhaustive).toUpperCase()
    }
  }
}
