import type { CredentialItemDto } from '@/api/control/types'

export function credentialDisplayName(item: CredentialItemDto): string {
  return item.label || item.account.email || item.account.email_mask || item.mask
}
