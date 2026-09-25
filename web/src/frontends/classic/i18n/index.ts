import { createI18n } from 'vue-i18n'

import { appLocale, type AppLocale } from '@shared/preferences/locale'

export { type AppLocale } from '@shared/preferences/locale'
export type MessageNamespace =
  'import' | 'group' | 'access-keys' | 'monitor' | 'models' | 'model-prices' | 'settings'

type MessageTree = { [key: string]: string | MessageTree }
type MessageLoader = () => Promise<{ default: MessageTree }>

const namespaces: MessageNamespace[] = [
  'import',
  'group',
  'access-keys',
  'monitor',
  'models',
  'model-prices',
  'settings',
]
const coreLoader: MessageLoader = () => import('./locales/en-GB/core')
const namespaceLoaders: Record<MessageNamespace, MessageLoader> = {
  import: () => import('./locales/en-GB/import'),
  group: () => import('./locales/en-GB/group'),
  'access-keys': () => import('./locales/en-GB/access-keys'),
  monitor: () => import('./locales/en-GB/monitor'),
  models: () => import('./locales/en-GB/models'),
  'model-prices': () => import('./locales/en-GB/model-prices'),
  settings: () => import('./locales/en-GB/settings'),
}

function createI18nPlugin(messages: MessageTree) {
  return createI18n({
    legacy: false as const,
    locale: appLocale,
    fallbackLocale: appLocale,
    messages: { [appLocale]: messages },
  })
}

export interface AppI18n {
  plugin: ReturnType<typeof createI18nPlugin>
  getLocale(): AppLocale
  loadNamespaces(requested: readonly MessageNamespace[]): Promise<void>
  loadAll(): Promise<void>
}

function createController(messages: MessageTree, loaded: Set<string>): AppI18n {
  const plugin = createI18nPlugin(messages)
  const pending = new Map<string, Promise<void>>()
  let activeNamespaces: readonly MessageNamespace[] = []

  async function ensure(namespace: 'core' | MessageNamespace): Promise<void> {
    if (loaded.has(namespace)) return
    const existing = pending.get(namespace)
    if (existing) return existing
    const request = (async () => {
      const loader = namespace === 'core' ? coreLoader : namespaceLoaders[namespace]
      const module = await loader()
      plugin.global.mergeLocaleMessage(appLocale, module.default)
      loaded.add(namespace)
    })().finally(() => pending.delete(namespace))
    pending.set(namespace, request)
    return request
  }

  document.documentElement.lang = appLocale
  return {
    plugin,
    getLocale() {
      return appLocale
    },
    async loadNamespaces(requested) {
      activeNamespaces = [...new Set(requested)]
      await Promise.all(activeNamespaces.map(ensure))
    },
    async loadAll() {
      activeNamespaces = namespaces
      await Promise.all(namespaces.map(ensure))
    },
  }
}

export async function createAppI18n(): Promise<AppI18n> {
  const core = (await coreLoader()).default
  return createController(core, new Set(['core']))
}
