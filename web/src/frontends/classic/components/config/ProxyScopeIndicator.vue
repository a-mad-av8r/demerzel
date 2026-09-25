<script setup lang="ts">
import { Route } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ProxyViewDto } from '@/api/control/types'
import AppTooltip from '@/components/ui/AppTooltip.vue'

const props = withDefaults(defineProps<{ view: ProxyViewDto; clickable?: boolean }>(), {
  clickable: false,
})
const emit = defineEmits<{ activate: [] }>()
const { t } = useI18n()

// Show an indicator only when the credential configures its own proxy rather than inheriting, avoiding visual noise.
const own = computed(() => props.view.configured_mode !== 'inherit')
// Report only the type; the expanded proxy panel owns the address to avoid exposing or crowding the list.
const tooltip = computed(() => {
  const type = t(`common.proxy.mode.${props.view.configured_mode}`)
  return props.clickable
    ? t('common.proxy.ownTooltipClickable', { type })
    : t('common.proxy.ownTooltip', { type })
})
</script>

<template>
  <AppTooltip v-if="own" :content="tooltip">
    <button
      v-if="clickable"
      class="proxy-scope-indicator proxy-scope-indicator--clickable"
      type="button"
      :aria-label="tooltip"
      @click="emit('activate')"
    >
      <Route :size="13" aria-hidden="true" />
    </button>
    <span v-else class="proxy-scope-indicator" tabindex="0" :aria-label="tooltip">
      <Route :size="13" aria-hidden="true" />
    </span>
  </AppTooltip>
</template>

<style scoped>
.proxy-scope-indicator {
  display: inline-grid;
  width: 22px;
  height: 22px;
  flex: none;
  place-items: center;
  border-radius: var(--radius-tag);
  background: var(--color-info-bg);
  color: var(--color-info);
  cursor: help;
}

/* The container is a fixed 22×22 border-box; user-agent padding would squeeze the icon. */
.proxy-scope-indicator--clickable {
  border: 0;
  padding: 0;
  font: inherit;
  cursor: pointer;
}

.proxy-scope-indicator:focus-visible {
  outline: 2px solid var(--color-focus);
  outline-offset: 2px;
}
</style>
