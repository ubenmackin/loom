import { relativeTime } from './relativeTime'

describe('relativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns "just now" for diff < 60 seconds', () => {
    vi.setSystemTime(new Date('2025-01-15T12:00:00Z'))
    expect(relativeTime('2025-01-15T12:00:00Z')).toBe('just now')
    expect(relativeTime('2025-01-15T11:59:45Z')).toBe('just now')
    expect(relativeTime('2025-01-15T11:59:01Z')).toBe('just now')
  })

  it('returns "just now" exactly at 0 seconds diff', () => {
    vi.setSystemTime(new Date('2025-01-15T12:00:00Z'))
    expect(relativeTime('2025-01-15T12:00:00Z')).toBe('just now')
  })

  it('returns "Xm ago" for diff < 60 minutes', () => {
    vi.setSystemTime(new Date('2025-01-15T12:05:00Z'))
    expect(relativeTime('2025-01-15T12:00:00Z')).toBe('5m ago')

    vi.setSystemTime(new Date('2025-01-15T12:59:59Z'))
    expect(relativeTime('2025-01-15T12:00:00Z')).toBe('59m ago')
  })

  it('returns "1m ago" exactly at 60 seconds (1 minute)', () => {
    vi.setSystemTime(new Date('2025-01-15T12:01:00Z'))
    expect(relativeTime('2025-01-15T12:00:00Z')).toBe('1m ago')
  })

  it('returns "Xh ago" for diff < 24 hours', () => {
    vi.setSystemTime(new Date('2025-01-15T15:00:00Z'))
    expect(relativeTime('2025-01-15T12:00:00Z')).toBe('3h ago')

    vi.setSystemTime(new Date('2025-01-16T11:59:59Z'))
    expect(relativeTime('2025-01-15T12:00:00Z')).toBe('23h ago')
  })

  it('returns "1h ago" exactly at 60 minutes (1 hour)', () => {
    vi.setSystemTime(new Date('2025-01-15T13:00:00Z'))
    expect(relativeTime('2025-01-15T12:00:00Z')).toBe('1h ago')
  })

  it('returns "Xd ago" for diff >= 24 hours', () => {
    vi.setSystemTime(new Date('2025-01-17T12:00:00Z'))
    expect(relativeTime('2025-01-15T12:00:00Z')).toBe('2d ago')

    vi.setSystemTime(new Date('2025-02-14T12:00:00Z'))
    expect(relativeTime('2025-01-15T12:00:00Z')).toBe('30d ago')
  })

  it('returns "1d ago" exactly at 24 hours', () => {
    vi.setSystemTime(new Date('2025-01-16T12:00:00Z'))
    expect(relativeTime('2025-01-15T12:00:00Z')).toBe('1d ago')
  })

  it('handles future dates (negative diffs) gracefully', () => {
    vi.setSystemTime(new Date('2025-01-15T12:00:00Z'))
    // Future date 5 minutes ahead
    expect(relativeTime('2025-01-15T12:05:00Z')).toBe('just now')
  })

  it('returns "Invalid date" for bad input', () => {
    vi.setSystemTime(new Date('2025-01-15T12:00:00Z'))
    expect(relativeTime('not-a-date')).toBe('Invalid date')
    expect(relativeTime('')).toBe('Invalid date')
    expect(relativeTime('2025-13-01T12:00:00Z')).toBe('Invalid date')
  })
})
