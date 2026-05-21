import type { StatusType } from '../types'

export function statusVariant(status: StatusType): 'default' | 'primary' | 'amber' | 'success' | 'error' {
  switch (status) {
    case 'done':
      return 'success'
    case 'blocked':
      return 'error'
    case 'in_progress':
      return 'amber'
    default:
      return 'default'
  }
}
