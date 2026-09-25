import type { SearchSelectOption } from './ui/types'

export function channelSearchOption(channel: {
  id: string
  name: string
  keywords?: readonly string[]
}): SearchSelectOption {
  return { value: channel.id, label: channel.name, keywords: channel.keywords ?? [] }
}
