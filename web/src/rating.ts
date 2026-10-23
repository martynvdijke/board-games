/** Format a 1–10 half-step rating for compact display. */
export function formatRating(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value) || value <= 0) return ''
  const clamped = Math.min(10, Math.max(1, value))
  const rounded = Math.round(clamped * 2) / 2
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1)
}

/** Convert a rating to a 0–5 star width percentage. */
export function ratingWidth(value: number | null | undefined): number {
  if (value == null || !Number.isFinite(value) || value <= 0) return 0
  return Math.min(100, Math.max(0, (value / 10) * 100))
}
