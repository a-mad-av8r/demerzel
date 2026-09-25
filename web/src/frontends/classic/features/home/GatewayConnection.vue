<script setup lang="ts">
import { Zap } from '@lucide/vue'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'

import { useApiClient } from '@shared/http/client-context'
import type { HomeBaseDto } from '@/app/resources/home'
import { accessKeysLocation } from '@/app/route-locations'
import { revealAccessKey } from '@/app/resources/access-keys'
import { registerEphemeralStateCleaner } from '@/app/ephemeral-state'
import { type ClipboardCopyResult, useClipboardCopy } from '@/app/use-clipboard-copy'
import AppButton from '@/components/ui/AppButton.vue'
import AppCombobox from '@/components/ui/AppCombobox.vue'
import AppConfirmDialog from '@/components/ui/AppConfirmDialog.vue'
import AppSelect from '@/components/ui/AppSelect.vue'
import CodeBlock from '@/components/ui/CodeBlock.vue'
import CopyAction from '@/components/ui/CopyAction.vue'
import CopyFallbackDialog from '@/components/ui/CopyFallbackDialog.vue'
import InlineFeedback from '@/components/ui/InlineFeedback.vue'
import ChannelIcon from '@/components/brand/ChannelIcon.vue'

import ClientPicker from './ClientPicker.vue'
import HomeSectionHeading from './HomeSectionHeading.vue'

import {
  ccSwitchTargets,
  clientConfiguration,
  clientFields,
  clientQuickImportURL,
  clientRequiredProtocol,
  gatewayClients,
  orderModelSuggestions,
  preferredGPTModel,
  type CCSwitchTargetID,
  type GatewayClientID,
} from './gateway-clients'

const props = defineProps<{
  accessKeys: HomeBaseDto['access_keys']
  selectedAccessKeyId: number | null
  clientId: GatewayClientID
  credential?: string
  selfScoped?: boolean
}>()
const emit = defineEmits<{
  'update:selectedAccessKeyId': [id: number]
  'update:clientId': [id: GatewayClientID]
}>()

type ActionTarget = 'key' | 'configuration' | 'quick-import' | 'baseUrl' | 'apiKey'
type FeedbackKind = 'success' | 'failure' | 'popup-blocked'

interface OperationIdentity {
  operationID: number
  accessKeyID: number
  clientID: GatewayClientID
}

interface ActionFeedback extends OperationIdentity {
  target: ActionTarget
  kind: FeedbackKind
}

const client = useApiClient()
const { t } = useI18n()
const { copy, fallbackText, reset: resetCopy } = useClipboardCopy()
const selectedKeyID = computed(() => props.selectedAccessKeyId)
const activeClient = computed(() => props.clientId)
const feedback = ref<ActionFeedback | null>(null)
const actionBusy = ref(false)
const quickImportConfirmationOpen = ref(false)
const ccSwitchTargetID = ref<CCSwitchTargetID>('claude')
const clientModel = ref('')
let resetTimer: number | undefined
let actionController: AbortController | undefined
let operationSequence = 0
let activeOperationID = 0
let unmounted = false

const origin = window.location.origin
const selectedKey = computed(
  () => props.accessKeys.find((accessKey) => accessKey.id === selectedKeyID.value) ?? null,
)
const selectOptions = computed(() =>
  props.accessKeys.map((accessKey) => ({
    value: String(accessKey.id),
    label: `${accessKey.name} · ${accessKey.masked_key}`,
  })),
)
const clientModelOptions = computed(() =>
  orderModelSuggestions(selectedKey.value?.models ?? []).map((model) => ({
    value: model,
    label: model,
  })),
)
const currentClient = computed(
  () =>
    gatewayClients.find((candidate) => candidate.id === activeClient.value) ?? gatewayClients[0]!,
)
const currentCCSwitchTarget = computed(
  () =>
    ccSwitchTargets.find((candidate) => candidate.id === ccSwitchTargetID.value) ??
    ccSwitchTargets[0]!,
)
const currentRequiredProtocol = computed(() =>
  clientRequiredProtocol(currentClient.value, currentCCSwitchTarget.value),
)
const selectedKeySupportsClient = computed(() => {
  const requiredProtocol = currentRequiredProtocol.value
  return Boolean(!requiredProtocol || selectedKey.value?.protocols.includes(requiredProtocol))
})
const quickImportAvailable = computed(() => Boolean(currentClient.value.quickImport))
const quickImportRequiresModel = computed(
  () => activeClient.value === 'cc-switch' && currentCCSwitchTarget.value.requiresModel,
)
const modelSelectionRequired = computed(
  () => quickImportRequiresModel.value || currentClient.value.requiresModel === true,
)
const quickImportReady = computed(
  () =>
    quickImportAvailable.value &&
    selectedKeySupportsClient.value &&
    (!quickImportRequiresModel.value || Boolean(clientModel.value.trim())),
)
const maskedSnippet = computed(() => {
  const key = selectedKey.value
  if (!key) return ''
  return clientConfiguration(
    activeClient.value,
    origin,
    key.masked_key,
    ccSwitchTargetID.value,
    clientModel.value,
    `Demerzel · ${key.name}`,
  )
})
const clientFieldList = computed(() => {
  const accessKey = selectedKey.value
  if (!accessKey || currentClient.value.configKind !== 'fields') return []
  return clientFields(activeClient.value, origin, accessKey.masked_key)
})

function fieldCopyState(id: ActionTarget): 'idle' | 'success' {
  return visibleFeedback.value?.target === id && visibleFeedback.value.kind === 'success'
    ? 'success'
    : 'idle'
}

async function copyField(field: { id: 'baseUrl' | 'apiKey'; secret?: boolean }): Promise<void> {
  if (!selectedKeySupportsClient.value) return
  if (!field.secret) {
    // Non-secret fields use their displayed values; decryption is unnecessary.
    const entry = clientFieldList.value.find((candidate) => candidate.id === field.id)
    if (!entry) return
    try {
      const result = await copy(entry.value)
      if (result === 'success') setImmediateFeedback(field.id, 'success')
    } catch {
      setImmediateFeedback(field.id, 'failure')
    }
    return
  }
  const clientID = activeClient.value
  await withRevealedKey(field.id, clientID, async (key, isCurrent) => {
    if (!isCurrent()) return
    return copy(key)
  })
}

const visibleFeedback = computed(() => {
  const current = feedback.value
  if (
    !current ||
    current.accessKeyID !== selectedKeyID.value ||
    current.clientID !== activeClient.value
  ) {
    return null
  }
  return current
})
const feedbackTone = computed(() => {
  if (visibleFeedback.value?.kind === 'success') return 'success'
  return 'danger'
})
const feedbackMessage = computed(() => {
  const current = visibleFeedback.value
  if (!current) return ''
  if (current.target === 'key') {
    return t(
      current.kind === 'success'
        ? 'home.ledger.connection.keyCopied'
        : 'home.ledger.connection.keyCopyFailed',
    )
  }
  if (current.target === 'configuration') {
    return t(
      current.kind === 'success'
        ? 'home.ledger.connection.configurationCopied'
        : 'home.ledger.connection.configurationCopyFailed',
    )
  }
  if (current.target === 'baseUrl' || current.target === 'apiKey') {
    const field = t(`home.ledger.connection.fields.${current.target}`)
    return current.kind === 'success'
      ? t('home.ledger.connection.fieldCopied', { field })
      : t('home.ledger.connection.fieldCopyFailed', { field })
  }
  if (current.kind === 'success') {
    return t('home.ledger.connection.quickImportRequested', {
      client: quickImportClientLabel.value,
    })
  }
  if (current.kind === 'popup-blocked') {
    return t('home.ledger.connection.popupBlocked', { client: quickImportClientLabel.value })
  }
  return t('home.ledger.connection.quickImportFailed', { client: quickImportClientLabel.value })
})
const configurationCopyState = computed(() =>
  visibleFeedback.value?.target === 'configuration' && visibleFeedback.value.kind === 'success'
    ? 'success'
    : 'idle',
)
const keyCopyState = computed(() =>
  visibleFeedback.value?.target === 'key' && visibleFeedback.value.kind === 'success'
    ? 'success'
    : 'idle',
)

function selectedClientLabel(clientID: GatewayClientID): string {
  return t(`home.ledger.connection.clients.${clientID}`)
}

function selectedClientKind(clientID: GatewayClientID): string {
  const kind = gatewayClients.find((candidate) => candidate.id === clientID)?.kind
  return kind ? t(`home.ledger.connection.clientKinds.${kind}`) : ''
}

const quickImportClientLabel = computed(() => {
  const clientLabel = selectedClientLabel(activeClient.value)
  if (activeClient.value !== 'cc-switch') return clientLabel
  return `${clientLabel} · ${t(`home.ledger.connection.ccSwitchTargets.${ccSwitchTargetID.value}`)}`
})

const configurationLanguage = computed(() => {
  if (activeClient.value === 'new-api') return t('home.ledger.connection.connectionInfo')
  if (activeClient.value === 'cc-switch') return t('home.ledger.connection.importParameters')
  if (activeClient.value === 'omp' || activeClient.value === 'openkai') return 'YAML'
  return t('home.ledger.connection.configuration')
})

const copyConfigurationLabel = computed(() => {
  if (activeClient.value === 'new-api') return t('home.ledger.connection.copyConnectionInfo')
  if (activeClient.value === 'cc-switch') return t('home.ledger.connection.copyImportParameters')
  return t('home.ledger.connection.copyConfiguration')
})

function identityMatches(identity: OperationIdentity): boolean {
  return (
    !unmounted &&
    activeOperationID === identity.operationID &&
    selectedKeyID.value === identity.accessKeyID &&
    activeClient.value === identity.clientID
  )
}

function operationIsCurrent(identity: OperationIdentity, controller: AbortController): boolean {
  return identityMatches(identity) && actionController === controller && !controller.signal.aborted
}

function setFeedback(identity: OperationIdentity, target: ActionTarget, kind: FeedbackKind): void {
  if (!identityMatches(identity)) return
  feedback.value = { ...identity, target, kind }
  window.clearTimeout(resetTimer)
  resetTimer = window.setTimeout(() => {
    if (feedback.value?.operationID === identity.operationID) feedback.value = null
  }, 2_000)
}

function setImmediateFeedback(target: ActionTarget, kind: FeedbackKind): void {
  const accessKey = selectedKey.value
  if (!accessKey || unmounted) return
  const identity = {
    operationID: ++operationSequence,
    accessKeyID: accessKey.id,
    clientID: activeClient.value,
  }
  activeOperationID = identity.operationID
  setFeedback(identity, target, kind)
}

function invalidateSensitiveAction(): void {
  resetCopy()
  activeOperationID = ++operationSequence
  actionController?.abort()
  actionController = undefined
  actionBusy.value = false
  feedback.value = null
  window.clearTimeout(resetTimer)
}

watch(
  () => props.accessKeys.map((accessKey) => accessKey.id),
  (ids) => {
    if (selectedKeyID.value !== null && ids.includes(selectedKeyID.value)) return
    invalidateSensitiveAction()
    quickImportConfirmationOpen.value = false
  },
  { immediate: true },
)

watch(
  selectedKeyID,
  (value, previous) => {
    if (value === previous) return
    invalidateSensitiveAction()
    quickImportConfirmationOpen.value = false
    clientModel.value = preferredGPTModel(selectedKey.value?.models ?? [])
    selectFirstSupportedCCSwitchTarget()
  },
  { immediate: true },
)

watch(activeClient, (value, previous) => {
  if (value === previous) return
  invalidateSensitiveAction()
  quickImportConfirmationOpen.value = false
})

watch(
  () => selectedKey.value?.protocols,
  () => selectFirstSupportedCCSwitchTarget(),
  { immediate: true },
)

watch(
  () => [ccSwitchTargetID.value, clientModel.value, props.credential],
  invalidateSensitiveAction,
  { flush: 'sync' },
)

function selectKey(value: string): void {
  if (actionBusy.value) return
  const id = Number(value)
  if (Number.isSafeInteger(id) && props.accessKeys.some((accessKey) => accessKey.id === id)) {
    emit('update:selectedAccessKeyId', id)
  }
}

function selectClient(value: string): void {
  if (actionBusy.value) return
  const gatewayClient = gatewayClients.find((candidate) => candidate.id === value)
  if (gatewayClient) emit('update:clientId', gatewayClient.id)
}

function selectCCSwitchTarget(value: string): void {
  if (actionBusy.value) return
  const target = ccSwitchTargets.find((candidate) => candidate.id === value)
  if (!target || !selectedKey.value?.protocols.includes(target.requiredProtocol)) return
  invalidateSensitiveAction()
  quickImportConfirmationOpen.value = false
  ccSwitchTargetID.value = target.id
  // Retain the model when switching targets; the same access key usually supports it.
}

function selectFirstSupportedCCSwitchTarget(): void {
  const protocols = selectedKey.value?.protocols ?? []
  if (protocols.includes(currentCCSwitchTarget.value.requiredProtocol)) return
  const supported = ccSwitchTargets.find((target) => protocols.includes(target.requiredProtocol))
  if (supported) ccSwitchTargetID.value = supported.id
}

async function withRevealedKey(
  target: ActionTarget,
  clientID: GatewayClientID,
  operation: (key: string, isCurrent: () => boolean) => Promise<ClipboardCopyResult | void> | void,
): Promise<boolean> {
  const accessKey = selectedKey.value
  if (!accessKey || actionBusy.value || unmounted || activeClient.value !== clientID) {
    return false
  }

  const identity = {
    operationID: ++operationSequence,
    accessKeyID: accessKey.id,
    clientID,
  }
  const controller = new AbortController()
  activeOperationID = identity.operationID
  actionController = controller
  actionBusy.value = true
  feedback.value = null
  window.clearTimeout(resetTimer)

  let secret: string | undefined
  const isCurrent = () => operationIsCurrent(identity, controller)
  try {
    secret = props.selfScoped
      ? props.credential
      : (await revealAccessKey(client, identity.accessKeyID, controller.signal)).key
    if (!secret) throw new Error('ACCESS_KEY_UNAVAILABLE')
    if (!isCurrent()) return false
    const result = await operation(secret, isCurrent)
    if (!isCurrent()) return false
    if (result !== undefined && result !== 'success') return false
    setFeedback(identity, target, 'success')
    return true
  } catch {
    if (isCurrent()) setFeedback(identity, target, 'failure')
    return false
  } finally {
    secret = undefined
    if (actionController === controller) {
      actionController = undefined
      actionBusy.value = false
    }
  }
}

async function copyAccessKey(): Promise<void> {
  const clientID = activeClient.value
  await withRevealedKey('key', clientID, async (key, isCurrent) => {
    if (!isCurrent()) return
    return copy(key)
  })
}

async function copyClientConfiguration(): Promise<void> {
  const clientID = activeClient.value
  if (!selectedKeySupportsClient.value) return

  if (clientID === 'codex' || clientID === 'omp' || clientID === 'openkai') {
    try {
      const result = await copy(maskedSnippet.value)
      if (result === 'success') setImmediateFeedback('configuration', 'success')
    } catch {
      setImmediateFeedback('configuration', 'failure')
    }
    return
  }

  await withRevealedKey('configuration', clientID, async (key, isCurrent) => {
    if (!isCurrent()) return
    let configuration: string | undefined
    try {
      configuration = clientConfiguration(
        clientID,
        origin,
        key,
        ccSwitchTargetID.value,
        clientModel.value,
        `Demerzel · ${selectedKey.value?.name ?? ''}`,
      )
      if (!isCurrent()) return
      return await copy(configuration)
    } finally {
      configuration = undefined
    }
  })
}

async function openQuickImport(): Promise<void> {
  const clientID = activeClient.value
  if (!quickImportReady.value || actionBusy.value) return

  const popup = window.open('about:blank', '_blank')
  if (!popup) {
    setImmediateFeedback('quick-import', 'popup-blocked')
    return
  }
  try {
    popup.opener = null
  } catch {
    popup.close()
    setImmediateFeedback('quick-import', 'failure')
    return
  }
  quickImportConfirmationOpen.value = false

  let target: string | undefined
  const opened = await withRevealedKey('quick-import', clientID, (key, isCurrent) => {
    if (!isCurrent()) return
    target =
      clientQuickImportURL(
        clientID,
        origin,
        key,
        ccSwitchTargetID.value,
        clientModel.value,
        `Demerzel · ${selectedKey.value?.name ?? ''}`,
      ) ?? undefined
    if (!target) throw new Error('QUICK_IMPORT_UNAVAILABLE')
    if (!isCurrent()) return
    popup.location.replace(target)
  })
  target = undefined
  if (!opened) popup.close()
}

const unregisterCopyCleaner = registerEphemeralStateCleaner(invalidateSensitiveAction)
onBeforeUnmount(() => {
  unmounted = true
  invalidateSensitiveAction()
  unregisterCopyCleaner()
})
</script>

<template>
  <section class="gateway-connection" aria-labelledby="gateway-connection-title">
    <HomeSectionHeading id="gateway-connection-title" :title="t('home.ledger.connection.title')" />

    <div v-if="!selectedKey" class="gateway-connection__empty">
      <p>{{ t('home.ledger.connection.noAccessKey') }}</p>
      <RouterLink v-if="!selfScoped" class="button-link" :to="accessKeysLocation()">
        {{ t('home.ledger.connection.createAccessKey') }}
      </RouterLink>
    </div>

    <template v-else>
      <div class="gateway-connection__toolbar">
        <div class="gateway-connection__key">
          <label class="gateway-connection__label" for="gateway-access-key">
            {{ t('home.ledger.connection.accessKey') }}
          </label>
          <div class="gateway-connection__key-control">
            <AppSelect
              id="gateway-access-key"
              variant="embedded"
              :model-value="String(selectedKey.id)"
              :label="t('home.ledger.connection.accessKey')"
              :options="selectOptions"
              :disabled="actionBusy || selfScoped"
              @update:model-value="selectKey"
            />
            <CopyAction
              variant="embedded"
              :label="t('home.ledger.connection.copyAccessKey')"
              :disabled="actionBusy"
              :busy="actionBusy"
              :state="keyCopyState"
              @copy="copyAccessKey"
            />
          </div>
        </div>

        <div class="gateway-connection__clients">
          <span id="gateway-client-label" class="gateway-connection__label">
            {{ t('home.ledger.connection.clients.label') }}
          </span>
          <ClientPicker
            :model-value="activeClient"
            :protocols="selectedKey.protocols"
            :disabled="actionBusy"
            @update:model-value="selectClient"
          />
        </div>
      </div>

      <div class="gateway-connection__panel">
        <header class="gateway-connection__panel-header">
          <span class="gateway-connection__panel-title">
            <strong>{{ selectedClientLabel(activeClient) }}</strong>
            <span v-if="selectedClientKind(activeClient)">
              {{ selectedClientKind(activeClient) }}
            </span>
          </span>
          <AppButton
            v-if="quickImportAvailable"
            size="compact"
            :disabled="!quickImportReady || actionBusy"
            :busy="actionBusy"
            @click="quickImportConfirmationOpen = true"
          >
            <Zap :size="14" aria-hidden="true" />
            {{
              t(
                activeClient === 'cc-switch'
                  ? 'home.ledger.connection.importAndEnable'
                  : 'home.ledger.connection.quickImport',
              )
            }}
          </AppButton>
        </header>

        <div class="gateway-connection__panel-body">
          <div
            v-if="activeClient === 'cc-switch' || currentClient.requiresModel"
            class="gateway-connection__model-options"
            :class="{ 'gateway-connection__model-options--single': activeClient !== 'cc-switch' }"
          >
            <div v-if="activeClient === 'cc-switch'" class="gateway-connection__cc-switch-target">
              <span class="gateway-connection__label">
                {{ t('home.ledger.connection.targetApplication') }}
              </span>
              <div
                class="gateway-connection__targets"
                role="group"
                :aria-label="t('home.ledger.connection.targetApplication')"
              >
                <button
                  v-for="target in ccSwitchTargets"
                  :key="target.id"
                  class="gateway-connection__target"
                  :class="{
                    'gateway-connection__target--selected': target.id === ccSwitchTargetID,
                  }"
                  type="button"
                  :disabled="actionBusy || !selectedKey.protocols.includes(target.requiredProtocol)"
                  :aria-pressed="target.id === ccSwitchTargetID"
                  @click="selectCCSwitchTarget(target.id)"
                >
                  <ChannelIcon :icon="target.icon" :mark="target.mark" />
                  <span>{{ t(`home.ledger.connection.ccSwitchTargets.${target.id}`) }}</span>
                </button>
              </div>
            </div>

            <div class="gateway-connection__model-picker">
              <span class="gateway-connection__label">
                {{ t('home.ledger.connection.primaryModel') }}
                <span v-if="modelSelectionRequired">
                  · {{ t('home.ledger.connection.required') }}
                </span>
              </span>
              <AppCombobox
                id="gateway-client-model"
                v-model="clientModel"
                :label="t('home.ledger.connection.primaryModel')"
                :options="clientModelOptions"
                :empty-text="t('home.ledger.connection.noModelMatches')"
                :placeholder="t('home.ledger.connection.modelPlaceholder')"
                :disabled="actionBusy"
                :maxlength="200"
                :spellcheck="false"
                monospace
                size="sm"
              />
            </div>
          </div>

          <InlineFeedback v-if="!selectedKeySupportsClient" tone="warning" appearance="hint">
            {{
              t('home.ledger.connection.protocolUnavailable', {
                client: quickImportClientLabel,
                protocol: currentRequiredProtocol,
              })
            }}
          </InlineFeedback>

          <InlineFeedback
            v-if="quickImportRequiresModel && !clientModel.trim()"
            tone="warning"
            appearance="hint"
          >
            {{ t('home.ledger.connection.ccSwitchModelRequired') }}
          </InlineFeedback>
          <InlineFeedback
            v-if="currentClient.requiresModel && !clientModel.trim()"
            tone="warning"
            appearance="hint"
          >
            {{ t('home.ledger.connection.modelRequired') }}
          </InlineFeedback>

          <!--
            The configuration is on the left; the matching setup steps are on the
            right. Clients configured only through a GUI use the full row for steps.
          -->
          <div class="gateway-connection__guide">
            <CodeBlock
              v-if="currentClient.configKind === 'snippet'"
              :code="maskedSnippet"
              :language="configurationLanguage"
              appearance="snippet"
            >
              <template #action>
                <CopyAction
                  :label="copyConfigurationLabel"
                  :disabled="
                    !selectedKeySupportsClient ||
                    actionBusy ||
                    (modelSelectionRequired && !clientModel.trim())
                  "
                  :busy="actionBusy"
                  :state="configurationCopyState"
                  @copy="copyClientConfiguration"
                />
              </template>
            </CodeBlock>

            <!-- GUI clients have no config file, so show their copyable fields in the left column. -->
            <section v-else class="gateway-connection__guide-config">
              <p class="gateway-connection__guide-caption">
                {{ t('home.ledger.connection.fieldsTitle') }}
              </p>
              <dl class="gateway-connection__fields">
                <div v-for="field in clientFieldList" :key="field.id">
                  <dt>{{ t(`home.ledger.connection.fields.${field.id}`) }}</dt>
                  <dd>
                    <code>{{ field.value }}</code>
                    <CopyAction
                      :label="
                        t('home.ledger.connection.copyField', {
                          field: t(`home.ledger.connection.fields.${field.id}`),
                        })
                      "
                      :disabled="!selectedKeySupportsClient || actionBusy"
                      :busy="field.secret ? actionBusy : false"
                      :state="fieldCopyState(field.id)"
                      @copy="copyField(field)"
                    />
                  </dd>
                </div>
              </dl>
            </section>

            <section class="gateway-connection__guide-steps">
              <p class="gateway-connection__guide-caption">
                {{ t('home.ledger.connection.stepsTitle') }}
              </p>
              <ol class="gateway-connection__steps">
                <li v-for="step in currentClient.steps" :key="step">
                  <span aria-hidden="true">{{ String(step).padStart(2, '0') }}</span>
                  <p>{{ t(`home.ledger.connection.steps.${activeClient}.s${step}`) }}</p>
                </li>
              </ol>
            </section>
          </div>
        </div>
      </div>
    </template>

    <InlineFeedback
      v-if="visibleFeedback"
      class="gateway-connection__feedback"
      :tone="feedbackTone"
      appearance="toast"
    >
      {{ feedbackMessage }}
    </InlineFeedback>

    <CopyFallbackDialog
      v-if="fallbackText !== undefined"
      :value="fallbackText"
      @close="resetCopy"
    />

    <AppConfirmDialog
      v-model:open="quickImportConfirmationOpen"
      :title="
        t('home.ledger.connection.quickImportConfirmTitle', { client: quickImportClientLabel })
      "
      :description="
        t('home.ledger.connection.quickImportConfirmDescription', {
          client: quickImportClientLabel,
        })
      "
      :close-label="t('common.close')"
      :cancel-label="t('common.cancel')"
      :confirm-label="
        t(
          activeClient === 'cc-switch'
            ? 'home.ledger.connection.importAndEnable'
            : 'home.ledger.connection.openAndImport',
        )
      "
      :pending="actionBusy"
      @confirm="openQuickImport"
    />
  </section>
</template>

<style scoped>
/*
 * Panel boundaries were too close to internal spacing to read as separate groups.
 * Widen the external gap and add a fine divider while keeping internal spacing compact.
 */
.gateway-connection {
  margin-top: 36px;
  border-top: 1px solid var(--color-border-subtle);
  padding-top: 20px;
}

.gateway-connection__toolbar {
  display: flex;
  align-items: flex-end;
  gap: 16px;
  flex-wrap: wrap;
  margin-top: 12px;
}

.gateway-connection__key {
  display: grid;
  min-width: 0;
  gap: 6px;
  flex: 0 1 300px;
}

.gateway-connection__label {
  color: var(--color-text-faint);
  font-size: var(--text-sm);
}

.gateway-connection__key-control {
  display: flex;
  min-width: 0;
  height: var(--control-md);
  overflow: hidden;
  border: 1px solid var(--color-border-control);
  border-radius: var(--radius-control);
  background: var(--color-surface);
}

/* Keep the two controls at the same height when they share a row. */
.gateway-connection__clients :deep(.client-picker__trigger) {
  min-height: var(--control-md);
}

/* Align the client and access-key controls as labelled fields in one row. */
.gateway-connection__clients {
  display: grid;
  min-width: 0;
  gap: 6px;
}

.gateway-connection__panel {
  display: grid;
  gap: var(--space-3);
  margin-top: 12px;
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-sheet);
  background: var(--color-surface);
  padding: 12px;
}

/*
 * Match the section heading used by the monitoring and health pages without a heavy
 * grey bar behind the title and code panel.
 */
.gateway-connection__panel-header {
  display: flex;
  /* Keep the title bar height stable when the quick-import button is absent. */
  min-height: var(--control-lg);
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  flex-wrap: wrap;
  border-radius: var(--radius-tag);
  background: color-mix(in srgb, var(--color-surface-sunken) 52%, var(--color-surface));
  padding: 6px 10px;
}

.gateway-connection__panel-title {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: var(--space-2-5);
}

.gateway-connection__panel-title strong {
  min-width: 0;
  font-size: var(--title-section);
  font-weight: 650;
  letter-spacing: -0.01em;
  line-height: var(--line-compact);
}

.gateway-connection__panel-title > span {
  color: var(--color-text-faint);
  font-size: var(--text-sm);
  font-weight: 400;
}

.gateway-connection__panel-body {
  display: grid;
  gap: var(--space-3);
  padding: 0 2px 2px;
}

.gateway-connection__model-options {
  display: grid;
  grid-template-columns: auto minmax(200px, 1fr);
  align-items: end;
  gap: var(--space-3);
}
.gateway-connection__model-options--single {
  grid-template-columns: minmax(200px, 1fr);
}

.gateway-connection__cc-switch-target,
.gateway-connection__model-picker {
  display: grid;
  min-width: 0;
  gap: 6px;
}

/* GUI clients show readable values with an independent copy action for each field. */
.gateway-connection__guide-config {
  min-width: 0;
}

.gateway-connection__fields {
  display: grid;
  flex: 1;
  align-content: start;
  margin: 0;
  gap: var(--space-2);
}

.gateway-connection__fields > div {
  display: grid;
  grid-template-columns: minmax(80px, max-content) minmax(0, 1fr);
  align-items: center;
  gap: var(--space-3);
}

.gateway-connection__fields dt {
  color: var(--color-text-faint);
  font-size: var(--text-sm);
}

.gateway-connection__fields dd {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: var(--space-2);
  margin: 0;
}

.gateway-connection__fields code {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-tag);
  background: var(--color-surface-sunken);
  color: var(--color-code);
  padding: 7px 10px;
  font-family: var(--font-mono);
  font-size: var(--text-sm);
  text-overflow: ellipsis;
  white-space: nowrap;
}

/*
 * Align the two columns at their captions. Stretch each body to fill its panel
 * without leaving excess space when one client has fewer setup steps.
 */
.gateway-connection__guide {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(240px, 320px);
  /* Stretch both columns to the same height and let each panel fill its body. */
  align-items: stretch;
  gap: var(--space-4);
}

/* Each column uses a caption and a body so both panels share the same alignment. */
.gateway-connection__guide > * {
  display: flex;
  min-height: 0;
  flex-direction: column;
}

.gateway-connection__guide :deep(.code-block--snippet pre) {
  flex: 1;
}

.gateway-connection__guide-steps {
  min-width: 0;
}

/*
 * Match the caption height and colour to the code panel; the copy control sets the
 * caption height because its toolbar has no minimum height.
 */
.gateway-connection__guide-caption {
  display: flex;
  min-height: 24px;
  align-items: center;
  margin: 0 0 6px;
  color: var(--color-text-faint);
  font-size: var(--text-sm);
}

/*
 * Reuse the numbered steps and panel background from the home welcome section.
 * Short instructions guide users through each step without overwhelming them.
 */
.gateway-connection__steps {
  display: grid;
  flex: 1;
  gap: var(--space-2);
  /* Keep steps top-aligned as the panel grows, with the remaining space filled by its background. */
  align-content: start;
  margin: 0;
  border-radius: var(--radius-tag);
  background: color-mix(in srgb, var(--color-surface-sunken) 52%, var(--color-surface));
  padding: 11px 13px;
  list-style: none;
}

.gateway-connection__steps li {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: baseline;
  gap: var(--space-2-5);
}

.gateway-connection__steps li > span {
  color: var(--color-text-faint);
  font-family: var(--font-mono);
  font-size: var(--text-label-xs);
  font-variant-numeric: tabular-nums;
  font-weight: 700;
}

.gateway-connection__steps p {
  margin: 0;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
  line-height: var(--line-editorial);
}

/* Keep the model input aligned with the target controls. */
.gateway-connection__model-picker :deep([data-input-shell]) {
  min-height: var(--control-sm);
  height: var(--control-sm);
}

/* Match the target-application chips to the channel picker used by import flows. */
.gateway-connection__targets {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}

.gateway-connection__target {
  display: inline-flex;
  min-height: var(--control-sm);
  align-items: center;
  gap: 8px;
  border: 1px solid var(--color-border-control);
  border-radius: var(--radius-control);
  background: var(--color-surface);
  color: var(--color-text);
  padding: 0 13px;
  font: inherit;
  font-size: var(--text-button);
  font-weight: 560;
  white-space: nowrap;
  cursor: pointer;
  transition:
    border-color var(--duration-fast) var(--easing-standard),
    background-color var(--duration-fast) var(--easing-standard),
    color var(--duration-fast) var(--easing-standard);
}

.gateway-connection__target:hover:not(:disabled) {
  border-color: var(--color-text-faint);
}

.gateway-connection__target:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.gateway-connection__target--selected {
  border-color: var(--color-action);
  background: var(--color-action-soft);
  color: var(--color-action);
  font-weight: 620;
}

.gateway-connection__target :deep(.channel-icon) {
  width: 17px;
  min-width: 17px;
  justify-content: center;
  font-size: 16px;
}

.gateway-connection__target :deep(.channel-icon--fallback) {
  width: 17px;
  min-width: 17px;
  height: 14px;
  font-size: 7.5px;
}

/* Match the import parameters and setup steps to the client heading background. */
.gateway-connection__panel :deep(.code-block--snippet pre) {
  border: 0;
  background: color-mix(in srgb, var(--color-surface-sunken) 52%, var(--color-surface));
  line-height: 1.6;
}

.gateway-connection__panel :deep(.code-block--snippet code) {
  color: var(--color-text);
}

.gateway-connection__panel :deep(.code-block__toolbar) {
  color: var(--color-text-faint);
  font-family: var(--font-mono);
}

.gateway-connection__feedback {
  margin: 0;
}

.gateway-connection__empty {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
  margin-top: var(--space-4);
}

.gateway-connection__empty p {
  margin: 0;
  color: var(--color-text-muted);
}

@media (max-width: 860px) {
  .gateway-connection__key {
    flex: 1 1 100%;
  }

  .gateway-connection__guide {
    grid-template-columns: minmax(0, 1fr);
  }

  .gateway-connection__model-options {
    grid-template-columns: 1fr;
  }
}

/* Restore 44px touch targets on narrow screens after reducing desktop control height. */
@media (max-width: 560px) {
  .gateway-connection__cc-switch-target :deep(.segmented-control__trigger),
  .gateway-connection__model-picker :deep([data-input-shell]) {
    min-height: var(--touch-target);
  }
}
</style>
