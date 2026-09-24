import {
  inject,
  onMounted,
  onScopeDispose,
  provide,
  readonly,
  ref,
  type InjectionKey,
  type Ref,
} from 'vue'

import { getCurrentVersion } from '@modern/api/system'
import { RequestCancelledError } from '@shared/http/errors'

interface SystemStatusState {
  version: Readonly<Ref<string | null>>
  versionLoading: Readonly<Ref<boolean>>
}

function createSystemStatus(): SystemStatusState {
  const version = ref<string | null>(null)
  const versionLoading = ref(false)
  const controller = new AbortController()

  async function loadVersion(): Promise<void> {
    if (versionLoading.value) return
    versionLoading.value = true
    try {
      version.value = await getCurrentVersion(controller.signal)
    } catch (error) {
      if (error instanceof RequestCancelledError) return
      version.value = null
    } finally {
      versionLoading.value = false
    }
  }

  onMounted(() => {
    void loadVersion()
  })
  onScopeDispose(() => controller.abort())

  return {
    version: readonly(version),
    versionLoading: readonly(versionLoading),
  }
}

const systemStatusKey: InjectionKey<SystemStatusState> = Symbol('modern-system-status')

export function provideSystemStatus(): void {
  provide(systemStatusKey, createSystemStatus())
}

export function useSystemStatus() {
  const status = inject(systemStatusKey)
  if (!status) throw new Error('MODERN_SYSTEM_STATUS_NOT_PROVIDED')
  return status
}
