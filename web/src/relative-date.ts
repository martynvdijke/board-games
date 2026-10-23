const DAY = 86_400_000

/** Human-friendly relative date for collections and play logs. */
export function relativeDate(value: string, now: Date = new Date()): string {
  const then = new Date(`${value.slice(0, 10)}T12:00:00`)
  if (Number.isNaN(then.getTime())) return value
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 12)
  const days = Math.round((startOfToday.getTime() - then.getTime()) / DAY)
  if (days < 0) return then.toLocaleDateString(undefined, { day: 'numeric', month: 'short' })
  if (days === 0) return 'today'
  if (days === 1) return 'yesterday'
  if (days < 7) return `${days} days ago`
  if (days < 30) {
    const weeks = Math.floor(days / 7)
    return weeks === 1 ? 'last week' : `${weeks} weeks ago`
  }
  if (days < 365) {
    const months = Math.floor(days / 30)
    return months === 1 ? 'last month' : `${months} months ago`
  }
  const years = Math.floor(days / 365)
  return years === 1 ? 'last year' : `${years} years ago`
}
