import type {
  HealthExpiringResetCreditDto,
  HealthProblemCredentialDto,
  HealthQuotaCredentialDto,
} from '@/app/resources/health'

export type AttentionKind = 'blacklisted' | 'expiringResetCredit' | 'lowQuota'

export interface AttentionItem {
  kind: AttentionKind
  groupID: number
  groupName: string

  value: number

  resetAtMS?: number

  expiresAtMS?: number
}

export const attentionRowLimit = 2

export function collectAttentionItems(
  blacklisted: readonly HealthProblemCredentialDto[],
  expiringResetCredits: readonly HealthExpiringResetCreditDto[],
  lowQuota: readonly HealthQuotaCredentialDto[],
): AttentionItem[] {
  const byGroup = new Map<number, AttentionItem>()
  for (const credential of blacklisted) {
    const existing = byGroup.get(credential.group_id)
    if (existing) {
      existing.value += 1
      continue
    }
    byGroup.set(credential.group_id, {
      kind: 'blacklisted',
      groupID: credential.group_id,
      groupName: credential.group_name,
      value: 1,
    })
  }

  const items = [...byGroup.values()].sort((left, right) => right.value - left.value)

  const resetCreditByGroup = new Map<number, AttentionItem>()
  for (const credential of expiringResetCredits) {
    const existing = resetCreditByGroup.get(credential.group_id)
    if (existing) {
      existing.value += credential.count
      existing.expiresAtMS = Math.min(
        existing.expiresAtMS ?? credential.nearest_expires_at_ms,
        credential.nearest_expires_at_ms,
      )
      continue
    }
    resetCreditByGroup.set(credential.group_id, {
      kind: 'expiringResetCredit',
      groupID: credential.group_id,
      groupName: credential.group_name,
      value: credential.count,
      expiresAtMS: credential.nearest_expires_at_ms,
    })
  }
  const resetCreditItems = [...resetCreditByGroup.values()].sort(
    (left, right) => (left.expiresAtMS ?? 0) - (right.expiresAtMS ?? 0),
  )

  const quotaItems = [...lowQuota]
    .sort((left, right) => left.remaining - right.remaining)
    .map<AttentionItem>((credential) => ({
      kind: 'lowQuota',
      groupID: credential.group_id,
      groupName: credential.group_name,
      value: credential.remaining,
      resetAtMS: credential.reset_at_ms,
    }))

  return [...items, ...resetCreditItems, ...quotaItems]
}

export function attentionTotal(items: readonly AttentionItem[]): number {
  return items.reduce((total, item) => total + (item.kind === 'lowQuota' ? 1 : item.value), 0)
}
