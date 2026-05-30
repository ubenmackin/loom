import { Status, type StatusType } from '../types'

export const STATUS_ORDER: StatusType[] = [
  Status.New,
  Status.Ready,
  Status.InProgress,
  Status.Blocked,
  Status.Done,
  Status.Canceled,
  Status.Archived,
]

/** Module-level constant for status display labels */
export const STATUS_LABELS: Record<string, string> = {
  [Status.New]: 'New',
  [Status.Ready]: 'Ready',
  [Status.InProgress]: 'In Progress',
  [Status.Blocked]: 'Blocked',
  [Status.Done]: 'Done',
  [Status.Canceled]: 'Canceled',
  [Status.Archived]: 'Archived',
}

export const VALID_TRANSITIONS: Record<StatusType, StatusType[]> = {
  [Status.New]: [Status.Ready, Status.InProgress, Status.Canceled],
  [Status.Ready]: [Status.InProgress, Status.Blocked, Status.Canceled],
  [Status.InProgress]: [Status.Done, Status.Blocked, Status.Canceled],
  [Status.Blocked]: [Status.Ready, Status.InProgress, Status.Canceled],
  [Status.Done]: [Status.Archived, Status.Canceled],
  [Status.Canceled]: [Status.New],
  [Status.Archived]: [],
}

export function statusVariant(status: StatusType): 'default' | 'primary' | 'amber' | 'success' | 'error' {
  switch (status) {
    case Status.Done:
      return 'success'
    case Status.Blocked:
      return 'error'
    case Status.InProgress:
      return 'amber'
    default:
      return 'default'
  }
}

export function statusDotClass(status: StatusType): string {
  switch (status) {
    case Status.InProgress:
      return 'status-dot status-dot-warning status-dot-pulse'
    case Status.Blocked:
      return 'status-dot status-dot-error'
    case Status.Done:
      return 'status-dot status-dot-success'
    case Status.New:
    case Status.Ready:
    case Status.Canceled:
    case Status.Archived:
    default:
      return 'status-dot status-dot-info'
  }
}

/** Shared staleness check — consistent threshold for both Story and Task */
export function isStale(updatedAt: string): boolean {
  const hours = (Date.now() - new Date(updatedAt).getTime()) / (1000 * 60 * 60)
  return hours > 2
}