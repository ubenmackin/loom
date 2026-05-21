import { Status, type StatusType } from '../types'

export const STATUS_ORDER: StatusType[] = [
  Status.New,
  Status.Ready,
  Status.InProgress,
  Status.Blocked,
  Status.Done,
]

export const VALID_TRANSITIONS: Record<string, StatusType[]> = {
  new: ['ready', 'in_progress'],
  ready: ['in_progress'],
  in_progress: ['done', 'blocked'],
  blocked: ['ready', 'in_progress'],
  done: [],
}
