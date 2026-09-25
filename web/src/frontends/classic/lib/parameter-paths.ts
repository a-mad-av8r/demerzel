import type { ParameterJSONValue } from '@/api/control/types'

export interface ParameterPathEntry {
  path: string
  value: ParameterJSONValue
}

class ParameterPathError extends Error {}

function escapeSegment(segment: string): string {
  return segment.replaceAll('~', '~0').replaceAll('/', '~1')
}

function unescapeSegment(raw: string): string | undefined {
  if (/~(?![01])/u.test(raw)) return undefined
  return raw.replaceAll('~1', '/').replaceAll('~0', '~')
}

function isPlainObject(value: ParameterJSONValue): value is Record<string, ParameterJSONValue> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function createParameterObject(): Record<string, ParameterJSONValue> {
  return Object.create(null) as Record<string, ParameterJSONValue>
}

function isNestedObject(value: ParameterJSONValue): value is Record<string, ParameterJSONValue> {
  return isPlainObject(value) && Object.keys(value).length > 0
}

function encodeParameterPath(segments: readonly string[]): string {
  return segments.map(escapeSegment).join('/')
}

export function decodeParameterPath(
  path: string,
  operation: 'set' | 'remove',
): string[] | undefined {
  if (!path) return undefined
  const segments: string[] = []
  for (const raw of path.split('/')) {
    const decoded = unescapeSegment(raw)
    if (
      decoded === undefined ||
      decoded === '' ||
      (operation === 'remove' && (decoded === '-' || /^\d+$/u.test(decoded)))
    )
      return undefined
    segments.push(decoded)
  }
  return segments
}

export function parameterPathsCross(left: string, right: string): boolean {
  return left === right || left.startsWith(`${right}/`) || right.startsWith(`${left}/`)
}

export function flattenParameterSet(set: Record<string, ParameterJSONValue>): ParameterPathEntry[] {
  const entries: ParameterPathEntry[] = []
  const walk = (node: Record<string, ParameterJSONValue>, prefix: readonly string[]): void => {
    for (const [key, value] of Object.entries(node)) {
      const segments = [...prefix, key]
      if (isNestedObject(value)) walk(value, segments)
      else entries.push({ path: encodeParameterPath(segments), value })
    }
  }
  walk(set, [])
  return entries
}

export function expandParameterSet(
  entries: readonly ParameterPathEntry[],
): Record<string, ParameterJSONValue> {
  const result = createParameterObject()
  for (const { path, value } of entries) {
    const segments = decodeParameterPath(path, 'set')
    const leaf = segments?.at(-1)
    if (!segments || leaf === undefined) throw new ParameterPathError(path)
    let node = result
    for (const segment of segments.slice(0, -1)) {
      const existing: ParameterJSONValue | undefined = node[segment]
      if (existing === undefined) node[segment] = createParameterObject()
      else if (!isPlainObject(existing)) throw new ParameterPathError(path)
      node = node[segment] as Record<string, ParameterJSONValue>
    }
    node[leaf] = value
  }
  return result
}

export function toParameterPointer(path: string): string {
  return `/${path}`
}

export function fromParameterPointer(pointer: string): string {
  return pointer.startsWith('/') ? pointer.slice(1) : pointer
}
