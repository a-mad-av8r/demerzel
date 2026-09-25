type Callback = () => void

let observer: ResizeObserver | undefined
const callbacks = new WeakMap<Element, Callback>()

export function observeOverflow(element: Element, callback: Callback): () => void {
  observer ??= new ResizeObserver((entries) => {
    for (const entry of entries) callbacks.get(entry.target)?.()
  })
  callbacks.set(element, callback)
  observer.observe(element)
  return () => {
    observer?.unobserve(element)
    callbacks.delete(element)
  }
}
