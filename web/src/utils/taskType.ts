import type { TaskTypeType } from '../types'

export function taskTypeLabel(type: TaskTypeType): string {
	switch (type) {
	case 'code':
		return 'CODE'
	case 'build':
		return 'BUILD'
	case 'review':
		return 'REVIEW'
	case 'planning':
		return 'PLANNING'
	case 'security':
		return 'SECURITY'
	case 'release':
		return 'RELEASE'
	case 'workspace_setup':
		return 'WORKSPACE'
	default: {
		const _exhaustive: never = type
		return String(_exhaustive).toUpperCase()
	}
	}
}

export function taskTypeVariant(type: TaskTypeType): 'default' | 'primary' | 'amber' | 'success' | 'error' {
  switch (type) {
    case 'code':
      return 'primary'
    case 'build':
      return 'amber'
    case 'review':
      return 'success'
    case 'security':
      return 'error'
    case 'release':
      return 'primary'
    case 'workspace_setup':
      return 'default'
    default: {
      return 'default'
    }
  }
}
