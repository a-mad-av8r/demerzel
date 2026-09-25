import { createI18n } from 'vue-i18n'

import { getAppLocale } from '@shared/preferences/locale'

import enGB from './locales/en-GB'

export function createModernI18n() {
  return createI18n({
    legacy: false,
    locale: getAppLocale(),
    fallbackLocale: 'en-GB',
    messages: { 'en-GB': enGB },
  })
}
