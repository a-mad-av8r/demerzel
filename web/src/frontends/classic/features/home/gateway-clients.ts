import type { AccessProtocol } from '@/api/control/types'

export type GatewayClientID =
  | 'cc-switch'
  | 'new-api'
  | 'codex'
  | 'omp'
  | 'openkai'
  | 'gemini-cli'
  | 'nextchat'
  | 'cherry-studio'
  | 'claude-code'
  | 'open-webui'
  | 'cline'
  | 'curl'

export type GatewayClientKind =
  | 'desktopManager'
  | 'gateway'
  | 'desktopWeb'
  | 'desktop'
  | 'web'
  | 'extension'
  | 'commandLine'
  | 'general'

/** Client groups are broader than client kinds to keep navigation compact. */
export type GatewayClientGroup = 'commandLine' | 'desktop' | 'web'

/**
 * Snippet clients have a real configuration file or importable text.
 * Field clients are configured in a GUI, so their values are shown separately
 * rather than presented as a file that does not exist.
 */
export type ClientConfigKind = 'snippet' | 'fields'

export interface ClientField {
  id: 'baseUrl' | 'apiKey'
  value: string
  secret?: boolean
}

export interface GatewayClient {
  id: GatewayClientID
  kind: GatewayClientKind
  configKind: ClientConfigKind
  /** Number of connection steps; message keys use steps.<id>.s1 … sN. */
  steps: number
  /**
   * Icon name in the classic client or channel assets. ChannelIcon falls back
   * to the mark when no matching file exists.
   */
  icon: string
  mark: string
  /** Search terms supplement the client name during filtering. */
  searchTerms: readonly string[]
  requiredProtocol?: AccessProtocol
  requiresModel?: boolean
  quickImport?: boolean
}

export function clientGroup(kind: GatewayClientKind): GatewayClientGroup {
  switch (kind) {
    case 'commandLine':
    case 'general':
      return 'commandLine'
    case 'desktopManager':
    case 'desktop':
    case 'desktopWeb':
      return 'desktop'
    default:
      return 'web'
  }
}

export type CCSwitchTargetID = 'codex' | 'claude' | 'gemini' | 'opencode'

export interface CCSwitchTarget {
  id: CCSwitchTargetID
  /** Uses the shared client-icon fallback from icon to mark. */
  icon: string
  mark: string
  requiredProtocol: AccessProtocol
  requiresModel: boolean
}

export const gatewayClients: readonly GatewayClient[] = [
  {
    id: 'cc-switch',
    kind: 'desktopManager',
    configKind: 'snippet',
    steps: 3,
    icon: 'cc-switch',
    mark: 'CC',
    searchTerms: ['ccswitch', 'switch'],
    quickImport: true,
  },
  {
    id: 'new-api',
    kind: 'gateway',
    configKind: 'snippet',
    steps: 3,
    icon: 'new-api',
    mark: 'NA',
    searchTerms: ['newapi', 'oneapi'],
  },
  {
    id: 'codex',
    kind: 'commandLine',
    configKind: 'snippet',
    steps: 3,
    icon: 'codex',
    mark: 'CX',
    searchTerms: ['openai', 'cli'],
    requiredProtocol: 'openai-responses',
  },
  {
    id: 'omp',
    kind: 'commandLine',
    configKind: 'snippet',
    steps: 3,
    icon: 'omp',
    mark: 'OM',
    searchTerms: ['oh-my-pi', 'oh my pi'],
    requiredProtocol: 'openai-responses',
    requiresModel: true,
  },
  {
    id: 'openkai',
    kind: 'commandLine',
    configKind: 'snippet',
    steps: 3,
    icon: 'openkai',
    mark: 'OK',
    searchTerms: ['open kai'],
    requiredProtocol: 'openai-responses',
    requiresModel: true,
  },
  {
    id: 'gemini-cli',
    kind: 'commandLine',
    configKind: 'snippet',
    steps: 2,
    icon: 'gemini-cli',
    mark: 'GC',
    searchTerms: ['gemini', 'google', 'cli'],
    requiredProtocol: 'gemini',
  },
  {
    id: 'nextchat',
    kind: 'desktopWeb',
    configKind: 'fields',
    steps: 2,
    icon: 'nextchat',
    mark: 'NC',
    searchTerms: ['nextchat', 'next-web'],
    requiredProtocol: 'openai-completions',
  },
  {
    id: 'cherry-studio',
    kind: 'desktop',
    configKind: 'fields',
    steps: 3,
    icon: 'cherry-studio',
    mark: 'CS',
    searchTerms: ['cherry', 'studio'],
    requiredProtocol: 'openai-completions',
    quickImport: true,
  },
  {
    id: 'claude-code',
    kind: 'commandLine',
    configKind: 'snippet',
    steps: 3,
    icon: 'claude',
    mark: 'CD',
    searchTerms: ['claude', 'anthropic', 'cli'],
    requiredProtocol: 'anthropic',
  },
  {
    id: 'open-webui',
    kind: 'web',
    configKind: 'fields',
    steps: 2,
    icon: 'open-webui',
    mark: 'OW',
    searchTerms: ['openwebui', 'ollama'],
    requiredProtocol: 'openai-completions',
  },
  {
    id: 'cline',
    kind: 'extension',
    configKind: 'fields',
    steps: 2,
    icon: 'cline',
    mark: 'CL',
    searchTerms: ['cline', 'roo', 'kilo', 'vscode'],
    requiredProtocol: 'openai-completions',
  },
  {
    id: 'curl',
    kind: 'general',
    configKind: 'snippet',
    steps: 2,
    icon: 'curl',
    mark: '>_',
    searchTerms: ['curl', 'shell', 'http'],
    requiredProtocol: 'openai-completions',
  },
]

export const ccSwitchTargets: readonly CCSwitchTarget[] = [
  {
    id: 'claude',
    icon: 'claude',
    mark: 'CD',
    requiredProtocol: 'anthropic',
    requiresModel: false,
  },
  {
    id: 'codex',
    icon: 'codex',
    mark: 'CX',
    requiredProtocol: 'openai-responses',
    requiresModel: true,
  },
  {
    id: 'gemini',
    icon: 'gemini-cli',
    mark: 'GC',
    requiredProtocol: 'gemini',
    requiresModel: false,
  },
  {
    id: 'opencode',
    icon: 'opencode',
    mark: 'OC',
    requiredProtocol: 'openai-completions',
    requiresModel: true,
  },
]

export function clientRequiredProtocol(
  client: GatewayClient,
  ccSwitchTarget: CCSwitchTarget,
): AccessProtocol | undefined {
  return client.id === 'cc-switch' ? ccSwitchTarget.requiredProtocol : client.requiredProtocol
}

const modelNameCollator = new Intl.Collator('en-GB', {
  numeric: true,
  sensitivity: 'base',
})

function gptVersion(model: string): number[] | null {
  const match = /^gpt-(\d+(?:\.\d+)*)/i.exec(model.trim())
  return match?.[1]?.split('.').map(Number) ?? null
}

function compareVersionsDescending(left: readonly number[], right: readonly number[]): number {
  const length = Math.max(left.length, right.length)
  for (let index = 0; index < length; index += 1) {
    const difference = (right[index] ?? -1) - (left[index] ?? -1)
    if (difference !== 0) return difference
  }
  return 0
}

/** GPT models first by newest numeric version, followed by the remaining model names. */
export function orderModelSuggestions(models: readonly string[]): string[] {
  return [...models].sort((left, right) => {
    const leftVersion = gptVersion(left)
    const rightVersion = gptVersion(right)
    if (leftVersion && rightVersion) {
      const versionOrder = compareVersionsDescending(leftVersion, rightVersion)
      if (versionOrder !== 0) return versionOrder
      const lengthOrder = left.length - right.length
      if (lengthOrder !== 0) return lengthOrder
    } else if (leftVersion) {
      return -1
    } else if (rightVersion) {
      return 1
    }
    return modelNameCollator.compare(left, right)
  })
}

export function preferredGPTModel(models: readonly string[]): string {
  return orderModelSuggestions(models).find((model) => gptVersion(model) !== null) ?? ''
}

export function clientConfiguration(
  clientID: GatewayClientID,
  origin: string,
  key: string,
  ccSwitchTarget: CCSwitchTargetID = 'claude',
  model = '',
  ccSwitchProviderName = 'Demerzel',
): string {
  switch (clientID) {
    case 'cc-switch': {
      const parameters: Record<string, string | boolean> = {
        app: ccSwitchTarget,
        name: ccSwitchProviderName,
        endpoint: ccSwitchEndpoint(origin, ccSwitchTarget),
        apiKey: key,
        enabled: true,
      }
      if (model.trim()) parameters.model = model.trim()
      return JSON.stringify(parameters, null, 2)
    }
    case 'new-api':
      return JSON.stringify(
        {
          _type: 'newapi_channel_conn',
          key,
          url: origin,
        },
        null,
        2,
      )
    case 'codex':
      return [
        'model_provider = "gpt-load"',
        '',
        '[model_providers.gpt-load]',
        'name = "Demerzel"',
        `base_url = "${openAIBaseURL(origin)}"`,
        'env_key = "GPT_LOAD_API_KEY"',
        'wire_api = "responses"',
      ].join('\n')
    case 'omp':
    case 'openkai': {
      const selectedModel = model.trim() || 'YOUR_MODEL'
      return [
        'providers:',
        '  demerzel:',
        `    baseUrl: ${JSON.stringify(openAIBaseURL(origin))}`,
        '    apiKey: DEMERZEL_API_KEY',
        '    api: openai-responses',
        '    models:',
        `      - id: ${JSON.stringify(selectedModel)}`,
        `        name: ${JSON.stringify(selectedModel)}`,
      ].join('\n')
    }
    case 'nextchat':
      return JSON.stringify({ url: origin, key }, null, 2)
    case 'cherry-studio':
      return JSON.stringify(
        {
          id: 'gpt-load',
          name: 'Demerzel',
          type: 'openai',
          baseUrl: openAIBaseURL(origin),
          apiKey: key,
        },
        null,
        2,
      )
    case 'gemini-cli':
      return [
        `export GOOGLE_GEMINI_BASE_URL="${origin.replace(/\/+$/, '')}"`,
        `export GEMINI_API_KEY="${key}"`,
      ].join('\n')
    case 'claude-code':
      return [
        `export ANTHROPIC_BASE_URL="${origin}"`,
        `export ANTHROPIC_AUTH_TOKEN="${key}"`,
        'export ANTHROPIC_MODEL="YOUR_MODEL"',
        'export ANTHROPIC_CUSTOM_MODEL_OPTION="$ANTHROPIC_MODEL"',
        'export CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY="1"',
      ].join('\n')
    case 'open-webui':
      return JSON.stringify(
        {
          url: openAIBaseURL(origin),
          apiKey: key,
        },
        null,
        2,
      )
    case 'cline':
      return JSON.stringify(
        {
          provider: 'OpenAI Compatible',
          baseUrl: openAIBaseURL(origin),
          apiKey: key,
          modelId: 'YOUR_MODEL',
        },
        null,
        2,
      )
    case 'curl':
      return [
        `curl "${openAIBaseURL(origin)}/chat/completions" \\`,
        `  -H "Authorization: Bearer ${key}" \\`,
        '  -H "Content-Type: application/json" \\',
        `  -d '{"model":"YOUR_MODEL","messages":[{"role":"user","content":"ping"}]}'`,
      ].join('\n')
  }
}

export function clientQuickImportURL(
  clientID: GatewayClientID,
  origin: string,
  key: string,
  ccSwitchTarget: CCSwitchTargetID = 'claude',
  model = '',
  ccSwitchProviderName = 'Demerzel',
): string | null {
  switch (clientID) {
    case 'cc-switch': {
      const params = new URLSearchParams({
        resource: 'provider',
        app: ccSwitchTarget,
        name: ccSwitchProviderName,
        homepage: origin,
        endpoint: ccSwitchEndpoint(origin, ccSwitchTarget),
        apiKey: key,
        enabled: 'true',
      })
      if (model.trim()) params.set('model', model.trim())
      return `ccswitch://v1/import?${params.toString()}`
    }
    case 'cherry-studio': {
      const payload = encodeURLSafeBase64(
        JSON.stringify({
          id: 'gpt-load',
          name: 'Demerzel',
          type: 'openai',
          baseUrl: openAIBaseURL(origin),
          apiKey: key,
        }),
      )
      return `cherrystudio://providers/api-keys?${new URLSearchParams({ v: '1', data: payload })}`
    }
    default:
      return null
  }
}

/**
 * GUI clients do not have a configuration file. Show their fields separately,
 * with independent copy actions for each value.
 */
export function clientFields(
  clientID: GatewayClientID,
  origin: string,
  key: string,
): ClientField[] {
  switch (clientID) {
    case 'cherry-studio':
    case 'open-webui':
    case 'cline':
      return [
        { id: 'baseUrl', value: openAIBaseURL(origin) },
        { id: 'apiKey', value: key, secret: true },
      ]
    case 'nextchat':
      return [
        { id: 'baseUrl', value: origin.replace(/\/+$/, '') },
        { id: 'apiKey', value: key, secret: true },
      ]
    default:
      return []
  }
}

function ccSwitchEndpoint(origin: string, target: CCSwitchTargetID): string {
  return target === 'codex' || target === 'opencode' ? openAIBaseURL(origin) : origin
}

function openAIBaseURL(origin: string): string {
  return `${origin.replace(/\/+$/, '')}/v1`
}

function encodeURLSafeBase64(value: string): string {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replaceAll('+', '_').replaceAll('/', '-')
}
