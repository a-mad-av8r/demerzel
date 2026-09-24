<script setup lang="ts">
import { Info } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { AppIcon, AppOverflowText, AppTooltip } from '@modern/components/ui'
import { useSystemStatus } from './useSystemStatus'

defineProps<{ collapsed?: boolean }>()

const { t } = useI18n()
const { version, versionLoading } = useSystemStatus()
const versionLabel = computed(() =>
  version.value
    ? version.value.startsWith('v')
      ? version.value
      : `v${version.value}`
    : t(versionLoading.value ? 'system.loadingVersion' : 'system.versionUnavailable'),
)
</script>

<template>
  <div class="modern-system-status" :class="{ 'is-compact': collapsed }">
    <div class="modern-version-row">
      <AppTooltip
        :label="t('system.currentVersion', { version: versionLabel })"
        :disabled="!collapsed"
        side="right"
      >
        <span class="modern-version" :tabindex="collapsed ? 0 : undefined">
          <AppIcon v-if="collapsed" :icon="Info" size="sm" />
          <AppOverflowText v-else :text="versionLabel" />
        </span>
      </AppTooltip>
    </div>
  </div>
</template>

<style scoped>
.modern-system-status {
  border-top: var(--modern-line-width) solid var(--modern-border);
  margin-top: var(--modern-space-2);
  padding: var(--modern-space-2) var(--modern-space-2) var(--modern-space-3);
  color: var(--modern-muted);
  font-size: var(--modern-font-size-caption);
}

.modern-version-row {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: var(--modern-space-1);
}

.modern-version {
  min-width: 0;
  overflow: hidden;
  font-family: var(--modern-font-mono);
  font-variant-numeric: tabular-nums;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.is-compact {
  padding-inline: 0;
}

.is-compact .modern-version-row {
  flex-direction: column;
  gap: 0;
}

.is-compact .modern-version {
  display: flex;
  width: var(--modern-control-nav);
  min-height: var(--modern-control-sm);
  align-items: center;
  justify-content: center;
}

</style>
