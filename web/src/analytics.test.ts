import { beforeEach, describe, expect, it, vi } from 'vitest'
import { track } from './analytics'

const umami = vi.fn()

beforeEach(() => {
  umami.mockReset()
  window.umami = { track: umami }
})

describe('track', () => {
  it('sends allowlisted properties', () => {
    expect(track('theme_changed', { theme: 'dark' })).toEqual({ theme: 'dark' })
    expect(umami).toHaveBeenCalledWith('theme_changed', { theme: 'dark' })
  })

  it('drops properties that are not allowlisted', () => {
    expect(track('play_submitted', { mode: 'create', game: 'Wingspan' })).toEqual({ mode: 'create' })
    expect(umami).toHaveBeenCalledWith('play_submitted', { mode: 'create' })
  })

  it('ignores unknown events', () => {
    expect(track('game_viewed', { name: 'Wingspan' })).toBeUndefined()
    expect(umami).not.toHaveBeenCalled()
  })

  it('rejects oversized and non-primitive values', () => {
    expect(track('theme_changed', { theme: 'x'.repeat(33) })).toEqual({})
    expect(track('shelf_search', { has_query: { evil: true } as unknown as boolean })).toEqual({})
  })

  it('does nothing when Umami is not configured', () => {
    delete window.umami
    expect(() => track('sync_started')).not.toThrow()
    expect(track('sync_started')).toEqual({})
  })
})
