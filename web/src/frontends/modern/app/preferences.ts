import { computed, inject, readonly, ref, type InjectionKey } from 'vue'

export const themes = ['system', 'light', 'dark'] as const
export type Theme = (typeof themes)[number]

function readPreference(key: string): string | null {
  try {
    return window.localStorage.getItem(key)
  } catch {
    return null
  }
}

export function createPreferences() {
  const storedTheme = readPreference('gpt-load.theme')
  const theme = ref<Theme>(
    themes.includes(storedTheme as Theme) ? (storedTheme as Theme) : 'system',
  )
  const systemScheme = window.matchMedia('(prefers-color-scheme: dark)')
  const systemDark = ref(systemScheme.matches)
  const resolvedTheme = computed<'light' | 'dark'>(() =>
    theme.value === 'system' ? (systemDark.value ? 'dark' : 'light') : theme.value,
  )
  function updateSystemTheme(event: MediaQueryListEvent): void {
    systemDark.value = event.matches
  }
  systemScheme.addEventListener('change', updateSystemTheme)
  const sidebarCollapsed = ref(readPreference('gpt-load.modern.sidebar-collapsed') === 'true')
  const persistenceFailed = ref(false)

  function persist(key: string, value: string): void {
    try {
      window.localStorage.setItem(key, value)
      if (window.localStorage.getItem(key) !== value) persistenceFailed.value = true
    } catch {
      // The selected interface preference remains available for this visit when storage is disabled.
      persistenceFailed.value = true
    }
  }

  function applyTheme(value: Theme): void {
    if (value === 'system') document.documentElement.removeAttribute('data-theme')
    else document.documentElement.dataset.theme = value
  }

  applyTheme(theme.value)

  return {
    theme: readonly(theme),
    resolvedTheme,
    sidebarCollapsed: readonly(sidebarCollapsed),
    persistenceFailed: readonly(persistenceFailed),
    dispose() {
      systemScheme.removeEventListener('change', updateSystemTheme)
    },
    setTheme(value: Theme) {
      theme.value = value
      applyTheme(value)
      persist('gpt-load.theme', value)
    },
    toggleSidebar() {
      sidebarCollapsed.value = !sidebarCollapsed.value
      persist('gpt-load.modern.sidebar-collapsed', String(sidebarCollapsed.value))
    },
  }
}

const preferencesKey: InjectionKey<ReturnType<typeof createPreferences>> =
  Symbol('modern-preferences')
export { preferencesKey }

export function usePreferences() {
  const preferences = inject(preferencesKey)
  if (!preferences) throw new Error('MODERN_PREFERENCES_NOT_PROVIDED')
  return preferences
}
