import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { findNavigationItem } from './navigation'

export function usePageTitle() {
  const route = useRoute()
  const { t } = useI18n()
  const current = computed(() => findNavigationItem(route.meta.primaryNav ?? route.name))
  const title = computed(() =>
    route.meta.titleKey
      ? t(route.meta.titleKey)
      : current.value
        ? t(`pages.${current.value.id}.title`)
        : t('notFound.title'),
  )
  return { current, title }
}
