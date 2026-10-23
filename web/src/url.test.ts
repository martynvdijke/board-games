import { describe, expect, it } from 'vitest'
import { shelfParams, shelfQuery } from './url'

function form(values: Record<string, string>): HTMLFormElement {
  const el = document.createElement('form')
  for (const [name, value] of Object.entries(values)) {
    const input = document.createElement('input')
    input.name = name
    input.value = value
    el.appendChild(input)
  }
  return el
}

describe('shelfQuery', () => {
  it('drops empty and default values', () => {
    const query = shelfQuery(form({ q: '  wingspan ', sort: 'name', view: 'grid', page: '1', status: '' }))
    expect(query).toBe('q=wingspan')
  })

  it('keeps meaningful filters and sorts keys', () => {
    const query = shelfQuery(form({ sort: 'plays', status: 'unplayed', q: 'brass' }))
    expect(query).toBe('q=brass&sort=plays&status=unplayed')
  })
})

describe('shelfParams', () => {
  it('omits empty values', () => {
    expect(shelfParams('?q=abc&status=&page=2')).toEqual({ q: 'abc', page: '2' })
  })
})
