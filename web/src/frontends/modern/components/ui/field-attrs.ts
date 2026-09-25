import type { StyleValue } from 'vue'

export function controlAttrs(attrs: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(attrs).filter(([key]) => key !== 'class' && key !== 'style'),
  )
}

export function layoutAttrs(attrs: Record<string, unknown>) {
  return { class: attrs.class, style: attrs.style as StyleValue | undefined }
}
