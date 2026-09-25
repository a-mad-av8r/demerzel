export const appLocale = 'en-GB' as const
export type AppLocale = typeof appLocale

export function getAppLocale(): AppLocale {
  return appLocale
}
