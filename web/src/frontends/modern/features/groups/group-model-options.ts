import type { GroupModel } from '@modern/api/group-detail'
import type { SearchSelectOption } from '@modern/components/ui'

export function groupValidationModelOptions(models: readonly GroupModel[]): SearchSelectOption[] {
  const aliases = new Map<string, Set<string>>()
  for (const model of models) {
    const names = aliases.get(model.id) ?? new Set<string>()
    if (model.alias) names.add(model.alias)
    aliases.set(model.id, names)
  }
  return [...aliases].map(([id, names]) => ({
    value: id,
    label: id,
    keywords: [...names],
    description: [...names].join(' · ') || undefined,
  }))
}
