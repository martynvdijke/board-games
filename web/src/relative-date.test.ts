import { describe, expect, it } from 'vitest'
import { relativeDate } from './relative-date'

const now = new Date('2026-10-23T18:00:00')

describe('relativeDate', () => {
  it('names today and yesterday', () => {
    expect(relativeDate('2026-10-23', now)).toBe('today')
    expect(relativeDate('2026-10-22', now)).toBe('yesterday')
  })

  it('counts days, weeks, months, and years', () => {
    expect(relativeDate('2026-10-20', now)).toBe('3 days ago')
    expect(relativeDate('2026-10-10', now)).toBe('last week')
    expect(relativeDate('2026-09-01', now)).toBe('last month')
    expect(relativeDate('2025-10-23', now)).toBe('last year')
  })

  it('returns the input when it cannot parse', () => {
    expect(relativeDate('not-a-date', now)).toBe('not-a-date')
  })
})
