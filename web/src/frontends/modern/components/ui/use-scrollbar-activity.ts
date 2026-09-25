import { onMounted, onScopeDispose } from 'vue'

export function useScrollbarActivity(): void {
  const active = new Map<HTMLElement, ReturnType<typeof setTimeout>>()
  let media: MediaQueryList | undefined

  function onScroll(event: Event): void {
    if (!media?.matches) return
    const target = event.target === document ? document.documentElement : event.target
    if (!(target instanceof HTMLElement)) return
    const timer = active.get(target)
    if (timer !== undefined) clearTimeout(timer)
    else target.setAttribute('data-modern-scroll-active', '')
    active.set(
      target,
      setTimeout(() => {
        target.removeAttribute('data-modern-scroll-active')
        active.delete(target)
      }, 700),
    )
  }

  onMounted(() => {
    if (!CSS.supports('selector(::-webkit-scrollbar)')) return
    media = window.matchMedia('(hover: hover) and (pointer: fine) and (forced-colors: none)')
    document.addEventListener('scroll', onScroll, { capture: true, passive: true })
  })

  onScopeDispose(() => {
    document.removeEventListener('scroll', onScroll, true)
    for (const [target, timer] of active) {
      clearTimeout(timer)
      target.removeAttribute('data-modern-scroll-active')
    }
    active.clear()
  })
}
