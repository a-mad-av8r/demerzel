import { onScopeDispose, readonly, ref } from 'vue'

const now = ref(Date.now())
let subscribers = 0
let timer: ReturnType<typeof setInterval> | undefined

export function useClock() {
  if (subscribers++ === 0) {
    now.value = Date.now()
    timer = setInterval(() => {
      now.value = Date.now()
    }, 30000)
  }
  onScopeDispose(() => {
    if (--subscribers === 0) {
      clearInterval(timer)
      timer = undefined
    }
  })
  return readonly(now)
}
