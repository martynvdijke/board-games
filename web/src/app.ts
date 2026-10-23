import { track } from './analytics'
import { debounce } from './debounce'
import { shelfQuery } from './url'

const CSRF = document.querySelector<HTMLMetaElement>('meta[name="csrf-token"]')?.content ?? ''

function resolvedTheme(): 'light' | 'dark' {
  const explicit = document.documentElement.dataset.theme
  if (explicit === 'light' || explicit === 'dark') return explicit
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function applyTheme(theme: string): void {
  if (theme === 'system') {
    document.documentElement.dataset.theme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
    document.documentElement.dataset.themePreference = 'system'
  } else {
    document.documentElement.dataset.theme = theme
    document.documentElement.dataset.themePreference = theme
  }
}

async function persistTheme(theme: string): Promise<void> {
  if (!CSRF) {
    document.cookie = `shelf_theme=${theme}; path=/; max-age=31536000; samesite=lax`
    return
  }
  const body = new FormData()
  body.set('theme', theme)
  try {
    await fetch('/settings/theme', {
      method: 'POST',
      headers: { Accept: 'application/json', 'X-CSRF-Token': CSRF },
      body,
    })
  } catch {
    /* The visual preference still applies for this page. */
  }
}

function initTheme(): void {
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (document.documentElement.dataset.themePreference === 'system') applyTheme('system')
  })

  document.querySelectorAll<HTMLButtonElement>('[data-theme-toggle]').forEach((button) => {
    button.addEventListener('click', () => {
      const next = resolvedTheme() === 'dark' ? 'light' : 'dark'
      applyTheme(next)
      track('theme_changed', { theme: next })
      void persistTheme(next)
    })
  })

  document.querySelectorAll<HTMLFormElement>('[data-theme-form]').forEach((form) => {
    form.addEventListener('submit', (event) => {
      const submitter = (event as SubmitEvent).submitter as HTMLButtonElement | null
      if (!submitter || submitter.name !== 'theme') return
      event.preventDefault()
      const theme = submitter.value
      applyTheme(theme)
      form.querySelectorAll<HTMLElement>('[data-theme-choice]').forEach((choice) => {
        choice.classList.toggle('is-active', choice.dataset.themeChoice === theme)
      })
      track('theme_changed', { theme })
      const body = new FormData(form)
      body.set('theme', theme)
      void fetch(form.action, {
        method: 'POST',
        headers: { Accept: 'application/json', 'X-CSRF-Token': CSRF },
        body,
      }).catch(() => undefined)
    })
  })
}

function initToast(): void {
  const toast = document.querySelector<HTMLElement>('[data-toast]')
  if (!toast) return
  const close = () => toast.remove()
  toast.querySelector('[data-toast-close]')?.addEventListener('click', close)
  window.setTimeout(close, 6000)
}

function initConfirmations(): void {
  document.addEventListener('submit', (event) => {
    const form = event.target as HTMLFormElement
    const message = form.dataset?.confirm
    if (message && !window.confirm(message)) {
      event.preventDefault()
    }
  })
}

function initCopy(): void {
  document.querySelectorAll<HTMLButtonElement>('[data-copy]').forEach((button) => {
    button.addEventListener('click', async () => {
      const target = document.querySelector<HTMLInputElement>('[data-copy-value]')
      if (!target) return
      try {
        await navigator.clipboard.writeText(target.value)
      } catch {
        target.select()
        document.execCommand('copy')
      }
      const original = button.innerHTML
      button.textContent = 'Copied'
      window.setTimeout(() => { button.innerHTML = original }, 1600)
    })
  })
}

function initFilters(): void {
  const form = document.querySelector<HTMLFormElement>('[data-filter-form]')
  const results = document.querySelector<HTMLElement>('#shelf-results')
  if (!form || !results) return
  const searchForm = form
  const resultPane = results

  const toggle = document.querySelector<HTMLButtonElement>('[data-filter-toggle]')
  const advanced = document.querySelector<HTMLElement>('#advanced-filters')
  toggle?.addEventListener('click', () => {
    if (!advanced) return
    const open = advanced.hidden
    advanced.hidden = !open
    toggle.setAttribute('aria-expanded', String(open))
  })

  const search = form.querySelector<HTMLInputElement>('[data-search]')
  const clear = form.querySelector<HTMLButtonElement>('[data-search-clear]')
  const syncClear = () => {
    if (clear && search) clear.hidden = search.value === ''
  }
  syncClear()
  search?.addEventListener('input', syncClear)
  clear?.addEventListener('click', () => {
    if (!search) return
    search.value = ''
    syncClear()
    search.focus()
    void load()
  })

  form.querySelectorAll<HTMLSelectElement>('[data-autosubmit]').forEach((select) => {
    select.addEventListener('change', () => form.requestSubmit())
  })

  let controller: AbortController | undefined
  let desired = 0

  async function load(submitter?: HTMLElement | null): Promise<void> {
    const request = ++desired
    controller?.abort()
    controller = new AbortController()
    const query = shelfQuery(searchForm, submitter)
    const url = `/partials/games${query ? `?${query}` : ''}`
    resultPane.setAttribute('aria-busy', 'true')
    try {
      const response = await fetch(url, {
        headers: { Accept: 'text/html', 'X-Requested-With': 'fetch' },
        signal: controller.signal,
      })
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
      const html = await response.text()
      if (request !== desired) return
      resultPane.innerHTML = html
      history.replaceState(null, '', `/${query ? `?${query}` : ''}`)
      const count = resultPane.querySelector('#result-count')?.textContent ?? ''
      resultPane.setAttribute('data-announcement', count)
    } catch (error) {
      if ((error as DOMException).name === 'AbortError') return
      window.location.assign(`/${query ? `?${query}` : ''}`)
    } finally {
      if (request === desired) resultPane.setAttribute('aria-busy', 'false')
    }
  }

  const debouncedLoad = debounce(() => void load(), 240)
  const trackSearch = debounce(() => track('shelf_search', { has_query: true }), 700)
  search?.addEventListener('input', (event) => {
    if ((event as InputEvent).inputType === 'insertCompositionText') return
    debouncedLoad()
    if (search.value.trim() !== '') trackSearch()
  })
  form.addEventListener('submit', (event) => {
    event.preventDefault()
    debouncedLoad.cancel()
    void load((event as SubmitEvent).submitter as HTMLElement | null)
  })
}

function initPlayerRows(): void {
  const container = document.querySelector<HTMLElement>('[data-player-rows]')
  const add = document.querySelector<HTMLButtonElement>('[data-add-player]')
  if (!container || !add) return

  const renumber = () => {
    container.querySelectorAll<HTMLElement>('[data-player-row]').forEach((row, index) => {
      const label = row.querySelector<HTMLElement>('.player-index')
      if (label) label.textContent = String(index + 1)
      row.querySelectorAll<HTMLInputElement>('input[name="player_winner"], input[name="player_first"]').forEach((input) => {
        input.value = String(index)
      })
    })
  }

  add.addEventListener('click', () => {
    const count = container.querySelectorAll('[data-player-row]').length
    if (count >= 20) return
    const row = document.createElement('div')
    row.className = 'player-row'
    row.dataset.playerRow = ''
    row.innerHTML = `
      <span class="player-index">${count + 1}</span>
      <label class="field"><span class="sr-only">Player name</span><input type="text" name="player_name" placeholder="Player name" autocomplete="off"></label>
      <label class="field field--score"><span class="sr-only">Score</span><input type="text" name="player_score" placeholder="Score"></label>
      <label class="check check--inline"><input type="checkbox" name="player_winner" value="${count}"><span>Won</span></label>
      <button class="icon-button icon-button--danger" type="button" data-remove-player aria-label="Remove player"><svg><use href="#i-close"/></svg></button>`
    container.appendChild(row)
    row.querySelector<HTMLInputElement>('input[name="player_name"]')?.focus()
  })

  container.addEventListener('click', (event) => {
    const button = (event.target as HTMLElement).closest('[data-remove-player]')
    if (!button) return
    const rows = container.querySelectorAll('[data-player-row]')
    if (rows.length <= 1) {
      rows.forEach((row) => row.querySelectorAll<HTMLInputElement>('input').forEach((input) => {
        if (input.type === 'checkbox') input.checked = false
        else input.value = ''
      }))
      return
    }
    button.closest('[data-player-row]')?.remove()
    renumber()
  })

  renumber()
}

interface SyncStatus {
  status: string
  running: boolean
  phase?: string
  collection_count?: number
  things_count?: number
  plays_count?: number
  error?: string
}

function initSyncStatus(): void {
  const box = document.querySelector<HTMLElement>('[data-sync-status]')
  if (!box || box.dataset.running !== 'true') return
  const pill = box.querySelector<HTMLElement>('[data-sync-pill]')
  const detail = box.querySelector<HTMLElement>('[data-sync-detail]')

  const poll = async (): Promise<void> => {
    try {
      const response = await fetch('/settings/sync/status', { headers: { Accept: 'application/json' } })
      if (!response.ok) return
      const status = (await response.json()) as SyncStatus
      if (pill) {
        pill.textContent = status.status
        pill.dataset.state = status.status
      }
      if (detail) {
        const parts = [
          status.phase ? `Phase: ${status.phase}` : '',
          `${status.collection_count ?? 0} collection`,
          `${status.things_count ?? 0} enriched`,
          `${status.plays_count ?? 0} plays`,
          status.error ?? '',
        ].filter(Boolean)
        detail.textContent = parts.join(' · ')
      }
      if (status.running) {
        window.setTimeout(() => void poll(), 4000)
      } else {
        box.dataset.running = 'false'
        window.setTimeout(() => window.location.reload(), 1200)
      }
    } catch {
      window.setTimeout(() => void poll(), 8000)
    }
  }

  window.setTimeout(() => void poll(), 2500)
}

function initAnalytics(): void {
  document.addEventListener('submit', (event) => {
    if (event.defaultPrevented) return
    const form = event.target as HTMLFormElement
    if (form.matches('[data-play-form]')) {
      track('play_submitted', { mode: form.dataset.playMode ?? 'create' })
    }
    if (form.matches('[data-sync-form]')) {
      track('sync_started')
    }
  })
}

export function init(): void {
  initTheme()
  initToast()
  initConfirmations()
  initAnalytics()
  initCopy()
  initFilters()
  initPlayerRows()
  initSyncStatus()
}
