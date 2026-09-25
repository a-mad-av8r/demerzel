import { shallowRef } from 'vue'

export const clipboardRevision = shallowRef(0)

export function canWriteToClipboardNatively(): boolean {
  return Boolean(
    globalThis.isSecureContext && typeof globalThis.navigator?.clipboard?.writeText === 'function',
  )
}

export async function copyText(
  value: string,
  target?: HTMLTextAreaElement,
  isCurrent: () => boolean = () => true,
): Promise<boolean> {
  if (!isCurrent()) return false
  const writeText = globalThis.navigator?.clipboard?.writeText
  if (canWriteToClipboardNatively() && typeof writeText === 'function') {
    try {
      await writeText.call(globalThis.navigator.clipboard, value)
      return true
    } catch {}
  }

  if (!isCurrent()) return false
  const activeElement = document.activeElement
  const textarea = target ?? document.createElement('textarea')
  try {
    if (target && !target.isConnected) return false
    textarea.value = value
    if (!target) {
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'

      const container =
        activeElement?.closest('[role="dialog"], [role="alertdialog"]') ?? document.body
      container.append(textarea)
    }
    textarea.focus({ preventScroll: true })
    textarea.select()
    return document.execCommand('copy')
  } catch {
    return false
  } finally {
    if (!target) {
      textarea.remove()
      if (activeElement instanceof HTMLElement && activeElement.isConnected) {
        activeElement.focus({ preventScroll: true })
      }
    }
  }
}
