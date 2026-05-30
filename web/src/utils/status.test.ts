import { describe, it, expect } from 'vitest'
import { Status, type StatusType } from '../types'
import {
  STATUS_ORDER,
  VALID_TRANSITIONS,
  STATUS_LABELS,
  statusVariant,
  statusDotClass,
  isStale,
} from './status'

const ALL_STATUSES: StatusType[] = [
  Status.New,
  Status.Ready,
  Status.InProgress,
  Status.Blocked,
  Status.Done,
  Status.Canceled,
  Status.Archived,
]

describe('status', () => {
  describe('STATUS_ORDER', () => {
    it('contains exactly 7 status entries', () => {
      expect(STATUS_ORDER).toHaveLength(7)
    })

    it('contains all status values in the correct order', () => {
      expect(STATUS_ORDER).toEqual(ALL_STATUSES)
    })
  })

  describe('VALID_TRANSITIONS', () => {
    it('has an entry for every status', () => {
      const keys = Object.keys(VALID_TRANSITIONS) as StatusType[]
      expect(keys).toHaveLength(7)
      ALL_STATUSES.forEach((status) => {
        expect(VALID_TRANSITIONS).toHaveProperty(status)
      })
    })

    it('every transition target is a valid status', () => {
      for (const targets of Object.values(VALID_TRANSITIONS)) {
        targets.forEach((target) => {
          expect(ALL_STATUSES).toContain(target)
        })
      }
    })
  })

  describe('statusVariant', () => {
    it.each([
      { status: Status.Done, expected: 'success' },
      { status: Status.Blocked, expected: 'error' },
      { status: Status.InProgress, expected: 'amber' },
      { status: Status.New, expected: 'default' },
      { status: Status.Ready, expected: 'default' },
      { status: Status.Canceled, expected: 'default' },
      { status: Status.Archived, expected: 'default' },
    ])('returns "$expected" for $status', ({ status, expected }) => {
      expect(statusVariant(status)).toBe(expected)
    })
  })

  describe('statusDotClass', () => {
    it.each([
      { status: Status.InProgress, expected: 'status-dot status-dot-warning status-dot-pulse' },
      { status: Status.Blocked, expected: 'status-dot status-dot-error' },
      { status: Status.Done, expected: 'status-dot status-dot-success' },
      { status: Status.New, expected: 'status-dot status-dot-info' },
      { status: Status.Ready, expected: 'status-dot status-dot-info' },
      { status: Status.Canceled, expected: 'status-dot status-dot-info' },
      { status: Status.Archived, expected: 'status-dot status-dot-info' },
    ])('returns "$expected" for $status', ({ status, expected }) => {
      expect(statusDotClass(status)).toBe(expected)
    })
  })

  describe('STATUS_LABELS', () => {
    it('has a label for every status', () => {
      ALL_STATUSES.forEach((status) => {
        expect(STATUS_LABELS[status]).toBeDefined()
        expect(typeof STATUS_LABELS[status]).toBe('string')
      })
    })

    it('maps in_progress to "In Progress"', () => {
      expect(STATUS_LABELS[Status.InProgress]).toBe('In Progress')
    })
  })

  describe('isStale', () => {
    it('returns true for timestamps older than 2 hours', () => {
      const threeHoursAgo = new Date(Date.now() - 3 * 60 * 60 * 1000).toISOString()
      expect(isStale(threeHoursAgo)).toBe(true)
    })

    it('returns false for recent timestamps', () => {
      const oneHourAgo = new Date(Date.now() - 1 * 60 * 60 * 1000).toISOString()
      expect(isStale(oneHourAgo)).toBe(false)
    })
  })
})
