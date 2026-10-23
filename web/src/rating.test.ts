import { describe, expect, it } from 'vitest'
import { formatRating, ratingWidth } from './rating'

describe('formatRating', () => {
  it('shows whole and half steps', () => {
    expect(formatRating(7)).toBe('7')
    expect(formatRating(7.5)).toBe('7.5')
    expect(formatRating(7.4)).toBe('7.5')
  })

  it('hides missing ratings', () => {
    expect(formatRating(null)).toBe('')
    expect(formatRating(0)).toBe('')
  })
})

describe('ratingWidth', () => {
  it('maps to a star percentage', () => {
    expect(ratingWidth(2.5)).toBe(25)
    expect(ratingWidth(10)).toBe(100)
    expect(ratingWidth(null)).toBe(0)
  })
})
