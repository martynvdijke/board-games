export type AnalyticsProps = Record<string, string | number | boolean>

const ALLOWED: Record<string, readonly string[]> = {
  theme_changed: ['theme'],
  shelf_search: ['has_query'],
  play_submitted: ['mode'],
  sync_started: [],
}

declare global {
  interface Window {
    umami?: {
      track: (event: string, data?: AnalyticsProps) => void
    }
  }
}

/**
 * Send a deliberately small, allowlisted event to Umami.
 *
 * Nothing from the page is forwarded by default. Callers may only supply the
 * named properties above; unknown events, keys, and value types are dropped.
 * Never pass game titles, notes, player names, usernames, or tokens.
 */
export function track(event: string, props: AnalyticsProps = {}): AnalyticsProps | undefined {
  const allowedKeys = ALLOWED[event]
  if (!allowedKeys) return undefined
  const safe: AnalyticsProps = {}
  for (const key of allowedKeys) {
    const value = props[key]
    if (typeof value === 'boolean' || typeof value === 'number') {
      safe[key] = value
    } else if (typeof value === 'string' && value.length <= 32) {
      safe[key] = value
    }
  }
  window.umami?.track(event, safe)
  return safe
}
