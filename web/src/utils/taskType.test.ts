import type { TaskTypeType } from '../types'
import { taskTypeLabel, taskTypeVariant } from './taskType'

describe('taskType', () => {
  describe('taskTypeLabel', () => {
    it('returns "CODE" for type "code"', () => {
      expect(taskTypeLabel('code')).toBe('CODE')
    })

    it('returns "BUILD" for type "build"', () => {
      expect(taskTypeLabel('build')).toBe('BUILD')
    })

    it('returns "REVIEW" for type "review"', () => {
      expect(taskTypeLabel('review')).toBe('REVIEW')
    })

    it('returns uppercased string for unknown type via exhaustiveness default branch', () => {
      expect(taskTypeLabel('unknown' as TaskTypeType)).toBe('UNKNOWN')
    })
  })

  describe('taskTypeVariant', () => {
    it('returns "primary" for type "code"', () => {
      expect(taskTypeVariant('code')).toBe('primary')
    })

    it('returns "amber" for type "build"', () => {
      expect(taskTypeVariant('build')).toBe('amber')
    })

    it('returns "success" for type "review"', () => {
      expect(taskTypeVariant('review')).toBe('success')
    })

    it('returns "default" for unknown type via exhaustiveness default branch', () => {
      expect(taskTypeVariant('unknown' as TaskTypeType)).toBe('default')
    })
  })
})
