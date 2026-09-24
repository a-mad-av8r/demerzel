<script setup lang="ts">
import type { SystemInfo } from '@modern/api/system'
import { AppBadge, AppButton, AppCollectionState, AppCopyValue } from '@modern/components/ui'
import { useI18n } from 'vue-i18n'

defineProps<{ data?: SystemInfo; loading: boolean; failed: boolean }>()
defineEmits<{ retry: [] }>()
const { t } = useI18n()
const databaseNames = { sqlite: 'SQLite', mysql: 'MySQL', postgres: 'PostgreSQL' }
</script>

<template>
  <AppCollectionState
    v-if="!data"
    :loading="loading"
    :error="failed"
    :title="t(loading ? 'settingsForm.loading' : 'settingsForm.system.failed')"
  >
    <AppButton v-if="failed" @click="$emit('retry')">{{ t('ui.retry') }}</AppButton>
  </AppCollectionState>
  <div v-else class="modern-settings-system">
    <div class="modern-settings-system-version">
      <span>{{ t('settingsForm.system.version') }}</span>
      <strong>{{ data.version }}</strong>
      <AppBadge size="xs">{{ t('settingsForm.system.deploymentValue') }}</AppBadge>
    </div>
    <dl class="modern-settings-system-facts">
      <div>
        <dt>{{ t('settingsForm.system.database') }}</dt>
        <dd>{{ databaseNames[data.database] }}</dd>
      </div>
      <div>
        <dt>{{ t('settingsForm.system.encryption') }}</dt>
        <dd>
          <AppBadge size="xs" tone="info">{{
            t('settingsForm.system.encryptionEnabled')
          }}</AppBadge>
        </dd>
      </div>
      <div class="modern-settings-system-wide">
        <dt>{{ t('settingsForm.system.dataDir') }}</dt>
        <dd><AppCopyValue :value="data.dataDir" /></dd>
      </div>
      <div
        v-for="key in ['authKey', 'encryption'] as const"
        :key="key"
        class="modern-settings-system-wide"
      >
        <dt>{{ t('settingsForm.system.' + key + 'Source') }}</dt>
        <dd>
          <AppBadge size="xs">{{ t('settingsForm.system.sources.' + data[key].source) }}</AppBadge>
          <AppCopyValue v-if="data[key].path" :value="data[key].path!" />
        </dd>
      </div>
    </dl>
    <p v-if="failed" class="modern-settings-system-note is-error" role="status">
      {{ t('settingsForm.stale') }}
    </p>
  </div>
</template>

<style scoped>
.modern-settings-system {
  display: grid;
  min-width: 0;
  gap: var(--modern-space-3);
}
.modern-settings-system-version {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  min-width: 0;
  gap: var(--modern-space-2) var(--modern-space-3);
}
.modern-settings-system-version > span {
  color: var(--modern-muted);
  font-size: var(--modern-font-size-secondary);
}
.modern-settings-system-version strong {
  font-size: var(--modern-font-size-section);
  font-weight: var(--modern-weight-semibold);
  overflow-wrap: anywhere;
}
.modern-settings-system-facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: var(--modern-space-5);
  min-width: 0;
  margin: 0;
}
.modern-settings-system-facts > div {
  display: grid;
  grid-template-columns: 144px minmax(0, 1fr);
  align-items: start;
  gap: var(--modern-space-3);
  min-width: 0;
  border-top: var(--modern-line-width) solid var(--modern-border);
  padding-block: var(--modern-space-3);
}
.modern-settings-system-facts dt {
  color: var(--modern-muted);
  font-size: var(--modern-font-size-secondary);
  overflow-wrap: anywhere;
}
.modern-settings-system-facts dd {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  min-width: 0;
  margin: 0;
  gap: var(--modern-space-2);
  font-size: var(--modern-font-size-secondary);
}
.modern-settings-system-wide {
  grid-column: 1 / -1;
}
.modern-settings-system-note {
  color: var(--modern-muted);
  font-size: var(--modern-font-size-small);
}
.modern-settings-system-note.is-error {
  color: var(--modern-danger);
}
@container modern-settings-content (max-width: 700px) {
  .modern-settings-system-facts {
    grid-template-columns: minmax(0, 1fr);
  }
}
@container modern-settings-content (max-width: 440px) {
  .modern-settings-system-facts > div {
    grid-template-columns: minmax(0, 1fr);
    gap: var(--modern-space-1);
  }
}
</style>
