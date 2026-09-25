import type { ParameterJSONValue } from '@/api/control/types'
import { assertJSONNumbersRoundTrip } from '@/lib/json-number'

export type ParameterValueKind = 'text' | 'number' | 'boolean' | 'null' | 'json'

export const parameterValueKinds: readonly ParameterValueKind[] = [
  'text',
  'number',
  'boolean',
  'null',
  'json',
]

export class ParameterValueError extends Error {}

const jsonNumberPattern = /^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?$/u

function isJSONNumberLiteral(text: string): boolean {
  return jsonNumberPattern.test(text)
}

export function hasEmptyParameterKey(value: unknown): boolean {
  if (Array.isArray(value)) return value.some(hasEmptyParameterKey)
  if (value === null || typeof value !== 'object') return false
  return Object.entries(value).some(([key, nested]) => key === '' || hasEmptyParameterKey(nested))
}

function needsQuoting(value: string): boolean {
  return (
    value === '' ||
    value.trim() !== value ||
    /[\u0000-\u001f]/u.test(value) ||
    value === 'true' ||
    value === 'false' ||
    value === 'null' ||
    isJSONNumberLiteral(value) ||
    value.startsWith('{') ||
    value.startsWith('[') ||
    value.startsWith('"')
  )
}

export function formatValueText(value: ParameterJSONValue): string {
  if (typeof value === 'string') return needsQuoting(value) ? JSON.stringify(value) : value
  return JSON.stringify(value) ?? 'null'
}

export function valueKind(value: ParameterJSONValue): ParameterValueKind {
  if (value === null) return 'null'
  if (typeof value === 'object') return 'json'
  switch (typeof value) {
    case 'number':
      return 'number'
    case 'boolean':
      return 'boolean'
    default:
      return 'text'
  }
}

export function inferValueKind(text: string): ParameterValueKind {
  const trimmed = text.trim()
  if (!trimmed) return 'text'
  if (trimmed.startsWith('{') || trimmed.startsWith('[')) return 'json'
  if (trimmed.startsWith('"')) return 'text'
  if (isJSONNumberLiteral(trimmed)) return 'number'
  if (trimmed === 'true' || trimmed === 'false') return 'boolean'
  if (trimmed === 'null') return 'null'
  return 'text'
}

function textValue(text: string): string {
  const trimmed = text.trim()
  if (trimmed.startsWith('"')) {
    try {
      const parsed: unknown = JSON.parse(trimmed)
      if (typeof parsed === 'string') return parsed
    } catch {}
  }
  return text
}

export function valueFromText(text: string, kind: ParameterValueKind): ParameterJSONValue {
  if (kind === 'null') return null
  const trimmed = text.trim()
  if (kind === 'text') return textValue(text)
  if (!trimmed) throw new ParameterValueError('empty')

  if (kind === 'number') {
    if (!isJSONNumberLiteral(trimmed)) throw new ParameterValueError('number')
    assertJSONNumbersRoundTrip(trimmed)
    return JSON.parse(trimmed) as number
  }
  if (kind === 'boolean') {
    if (trimmed === 'true') return true
    if (trimmed === 'false') return false
    throw new ParameterValueError('boolean')
  }

  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) {
    throw new ParameterValueError('json')
  }
  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    throw new ParameterValueError('json')
  }
  if (hasEmptyParameterKey(parsed)) throw new ParameterValueError('empty-key')
  assertJSONNumbersRoundTrip(trimmed)
  return parsed as ParameterJSONValue
}

export function tryValueFromText(
  text: string,
  kind: ParameterValueKind,
): ParameterJSONValue | undefined {
  try {
    return valueFromText(text, kind)
  } catch {
    return undefined
  }
}
