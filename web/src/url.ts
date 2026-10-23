/** Build a canonical shelf query string from a form. Empty values are omitted. */
export function shelfQuery(form: HTMLFormElement, submitter?: HTMLElement | null): string {
  const data = new FormData(form, submitter ?? undefined)
  const params = new URLSearchParams()
  for (const [key, raw] of data.entries()) {
    if (typeof raw !== 'string') continue
    const value = raw.trim()
    if (!value) continue
    params.append(key, value)
  }
  const page = params.get('page')
  if (page === '1') params.delete('page')
  const sort = params.get('sort')
  if (sort === 'name') params.delete('sort')
  const view = params.get('view')
  if (view === 'grid') params.delete('view')
  params.sort()
  return params.toString()
}

/** Parse a location search value into the same shape the server understands. */
export function shelfParams(search: string): Record<string, string> {
  const params = new URLSearchParams(search)
  const out: Record<string, string> = {}
  params.forEach((value, key) => {
    if (value !== '') out[key] = value
  })
  return out
}
