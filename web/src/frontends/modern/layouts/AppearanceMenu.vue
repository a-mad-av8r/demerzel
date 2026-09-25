<script setup lang="ts">
import { Monitor, Moon, Sun } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { themes, usePreferences, type Theme } from '@modern/app/preferences'
import { AppSelectMenu } from '@modern/components/ui'

const { t } = useI18n()
const { theme, setTheme } = usePreferences()
const themeIcon = computed(() => ({ system: Monitor, light: Sun, dark: Moon })[theme.value])
const themeOptions = computed(() =>
  themes.map((value) => ({ value, label: t(`appearance.themes.${value}`) })),
)

function changeTheme(value: string): void {
  if (themes.includes(value as Theme)) setTheme(value as Theme)
}
</script>

<template>
  <AppSelectMenu
    :label="t('appearance.theme')"
    :icon="themeIcon"
    :model-value="theme"
    :options="themeOptions"
    @update:model-value="changeTheme"
  />
</template>
