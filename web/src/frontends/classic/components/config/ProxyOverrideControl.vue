<script setup lang="ts">
import { computed, useId } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ProxyConfiguredMode, ProxyViewDto } from '@/api/control/types'
import { proxyDraftState, proxyPlaceholderURL } from '@/app/resources/proxy'
import AppTextInput from '@/components/ui/AppTextInput.vue'
import CompactFieldError from '@/components/ui/CompactFieldError.vue'
import SegmentedControl, { type SegmentedControlOption } from '@/components/ui/SegmentedControl.vue'

const props = withDefaults(
  defineProps<{
    /** Saved baseline used to derive the placeholder address and validation state. */
    base: ProxyViewDto
    mode: ProxyConfiguredMode
    endpoint: string
    disabled?: boolean
  }>(),
  { disabled: false },
)
const emit = defineEmits<{
  'update:mode': [value: ProxyConfiguredMode]
  'update:endpoint': [value: string]
}>()

const { t } = useI18n()
const inputId = `${useId()}-proxy-url`

// The outer “Override / restore defaults” toggle represents inherit; expose only two explicit override modes here.
const modeOptions = computed<SegmentedControlOption[]>(() => [
  { value: 'direct', label: t('common.proxy.mode.direct'), disabled: props.disabled },
  { value: 'custom', label: t('common.proxy.mode.custom'), disabled: props.disabled },
])
const error = computed(() =>
  proxyDraftState(props.base, props.mode, props.endpoint).invalid
    ? t('common.proxy.invalid')
    : undefined,
)
const placeholder = computed(() => proxyPlaceholderURL(props.base) ?? t('common.proxy.placeholder'))

function setMode(value: string): void {
  if (props.disabled || (value !== 'direct' && value !== 'custom')) return
  emit('update:mode', value)
}
</script>

<template>
  <div class="proxy-override-control">
    <SegmentedControl
      :model-value="mode === 'custom' ? 'custom' : 'direct'"
      :label="t('common.proxy.modeLabel')"
      :options="modeOptions"
      size="compact"
      @update:model-value="setMode"
    />

    <CompactFieldError v-if="mode === 'custom'" :id="inputId" :error="error">
      <template #default="{ invalid, describedBy }">
        <AppTextInput
          :id="inputId"
          :model-value="endpoint"
          :label="t('common.proxy.urlLabel')"
          :placeholder="placeholder"
          appearance="surface"
          size="compact"
          autocomplete="off"
          :spellcheck="false"
          monospace
          :disabled="disabled"
          :invalid="invalid"
          :described-by="describedBy"
          @update:model-value="emit('update:endpoint', $event)"
        />
      </template>
    </CompactFieldError>
  </div>
</template>

<style scoped>
.proxy-override-control {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
}

.proxy-override-control > :deep(.compact-field-error) {
  flex: 1 1 190px;
  min-width: 0;
}
</style>
