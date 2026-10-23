export type Debounced<A extends unknown[]> = ((...args: A) => void) & { cancel: () => void }

export function debounce<A extends unknown[]>(fn: (...args: A) => void, wait = 220): Debounced<A> {
  let timer: ReturnType<typeof setTimeout> | undefined
  const debounced = (...args: A) => {
    if (timer !== undefined) clearTimeout(timer)
    timer = setTimeout(() => {
      timer = undefined
      fn(...args)
    }, wait)
  }
  debounced.cancel = () => {
    if (timer !== undefined) clearTimeout(timer)
    timer = undefined
  }
  return debounced
}
