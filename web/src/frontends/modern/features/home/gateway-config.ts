// Client metadata controls config generation and default protocol checks; it does not restrict access-key choices.
export const gatewayClients = [
  {
    id: 'codex',
    name: 'Codex',
    icon: 'codex',
    protocol: 'openai-responses',
    kind: 'snippet',
    surface: 'cli',
    group: 'cli',
  },
  {
    id: 'omp',
    name: 'OMP',
    icon: 'omp',
    protocol: 'openai-responses',
    kind: 'snippet',
    surface: 'cli',
    group: 'cli',
  },
  {
    id: 'openkai',
    name: 'OpenKai',
    icon: 'openkai',
    protocol: 'openai-responses',
    kind: 'snippet',
    surface: 'cli',
    group: 'cli',
  },
  {
    id: 'claude-code',
    name: 'Claude Code',
    icon: 'claude',
    protocol: 'anthropic',
    kind: 'snippet',
    surface: 'cli',
    group: 'cli',
  },
  {
    id: 'gemini-cli',
    name: 'Gemini CLI',
    icon: 'gemini',
    protocol: 'gemini',
    kind: 'snippet',
    surface: 'cli',
    group: 'cli',
  },
  {
    id: 'cline',
    name: 'Cline',
    icon: 'cline',
    protocol: 'openai-completions',
    kind: 'fields',
    surface: 'gui',
    group: 'cli',
  },
  {
    id: 'cc-switch',
    name: 'CC Switch',
    icon: 'cc-switch',
    protocol: '',
    kind: 'snippet',
    surface: 'gui',
    group: 'desktop',
  },
  {
    id: 'cherry-studio',
    name: 'Cherry Studio',
    icon: 'cherry-studio',
    protocol: 'openai-completions',
    kind: 'fields',
    surface: 'gui',
    group: 'desktop',
  },
  {
    id: 'nextchat',
    name: 'NextChat',
    icon: 'nextchat',
    protocol: 'openai-completions',
    kind: 'fields',
    surface: 'gui',
    group: 'desktop',
  },
  {
    id: 'open-webui',
    name: 'Open WebUI',
    icon: 'open-webui',
    protocol: 'openai-completions',
    kind: 'fields',
    surface: 'gui',
    group: 'desktop',
  },
  {
    id: 'new-api',
    name: 'New API',
    icon: 'new-api',
    protocol: '',
    kind: 'snippet',
    surface: 'gui',
    group: 'relay',
  },
  {
    id: 'curl',
    name: 'cURL',
    icon: 'curl',
    protocol: 'openai-completions',
    kind: 'snippet',
    surface: 'cli',
    group: 'relay',
  },
] as const
export const gatewayGroups = ['cli', 'desktop', 'relay'] as const
export type GatewayGroupID = (typeof gatewayGroups)[number]
export type GatewayClientID = (typeof gatewayClients)[number]['id']
export const gatewayTargets = [
  { id: 'claude', name: 'Claude Code', protocol: 'anthropic', requiresModel: false },
  { id: 'codex', name: 'Codex', protocol: 'openai-responses', requiresModel: true },
  { id: 'gemini', name: 'Gemini CLI', protocol: 'gemini', requiresModel: false },
  { id: 'opencode', name: 'OpenCode', protocol: 'openai-completions', requiresModel: true },
] as const
export type GatewayTargetID = (typeof gatewayTargets)[number]['id']
export interface GatewayConfig {
  client: GatewayClientID
  origin: string
  model: string
  target: GatewayTargetID
  name: string
}
export interface ConfigBlock {
  label: string
  content: string
  /** True when copying this block must resolve an access key. */
  requiresKey?: boolean
}
const shellQuote = (value: string) => "'" + value.replaceAll("'", "'\\''") + "'"
const tomlQuote = (value: string) => JSON.stringify(value)
export function gatewayEndpoint(config: GatewayConfig): string {
  const root = config.origin.replace(/\/+$/, '')
  return ['claude-code', 'gemini-cli', 'nextchat', 'new-api'].includes(config.client) ||
    (config.client === 'cc-switch' && ['claude', 'gemini'].includes(config.target))
    ? root
    : root + '/v1'
}
export function gatewayConfiguration(config: GatewayConfig, key: string): ConfigBlock[] {
  const endpoint = gatewayEndpoint(config)
  const model = config.model.trim() || 'YOUR_MODEL'
  const env = (name: string, value: string) => `export ${name}=${shellQuote(value)}`
  switch (config.client) {
    case 'codex':
      return [
        {
          label: '~/.codex/config.toml',
          content: [
            `model = ${tomlQuote(model)}`,
            'model_provider = "gpt-load"',
            '',
            '[model_providers.gpt-load]',
            'name = "Demerzel"',
            `base_url = ${tomlQuote(endpoint)}`,
            'env_key = "GPT_LOAD_API_KEY"',
            'wire_api = "responses"',
          ].join('\n'),
        },
        { label: 'shell', content: env('GPT_LOAD_API_KEY', key), requiresKey: true },
      ]
    case 'omp':
    case 'openkai': {
      const envPath = config.client === 'omp' ? '~/.omp/agent/.env' : '~/.openkai/.env'
      return [
        {
          label: '~/.omp/agent/models.yml',
          content: [
            'providers:',
            '  demerzel:',
            `    baseUrl: ${JSON.stringify(endpoint)}`,
            '    apiKey: DEMERZEL_API_KEY',
            '    api: openai-responses',
            '    models:',
            `      - id: ${JSON.stringify(model)}`,
            `        name: ${JSON.stringify(model)}`,
          ].join('\n'),
        },
        {
          label: envPath,
          content: `DEMERZEL_API_KEY=${JSON.stringify(key)}`,
          requiresKey: true,
        },
      ]
    }
    case 'claude-code':
      return [
        {
          label: 'shell',
          content: [
            env('ANTHROPIC_BASE_URL', endpoint),
            env('ANTHROPIC_AUTH_TOKEN', key),
            env('ANTHROPIC_MODEL', model),
            'export ANTHROPIC_CUSTOM_MODEL_OPTION="$ANTHROPIC_MODEL"',
            'export CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY="1"',
          ].join('\n'),
          requiresKey: true,
        },
      ]
    case 'gemini-cli':
      return [
        {
          label: 'shell',
          content: [env('GOOGLE_GEMINI_BASE_URL', endpoint), env('GEMINI_API_KEY', key)].join('\n'),
          requiresKey: true,
        },
      ]
    case 'cc-switch':
      return [
        {
          label: 'JSON',
          content: JSON.stringify(
            {
              app: config.target,
              name: config.name,
              endpoint,
              apiKey: key,
              enabled: true,
              ...(config.model.trim() ? { model: config.model.trim() } : {}),
            },
            null,
            2,
          ),
          requiresKey: true,
        },
      ]
    case 'new-api':
      return [
        {
          label: 'JSON',
          content: JSON.stringify({ _type: 'newapi_channel_conn', key, url: endpoint }, null, 2),
          requiresKey: true,
        },
      ]
    case 'curl':
      return [
        {
          label: 'shell',
          content: [
            `curl ${shellQuote(endpoint + '/chat/completions')} \\`,
            `  -H ${shellQuote('Authorization: Bearer ' + key)} \\`,
            "  -H 'Content-Type: application/json' \\",
            `  -d ${shellQuote(JSON.stringify({ model, messages: [{ role: 'user', content: 'ping' }] }))}`,
          ].join('\n'),
          requiresKey: true,
        },
      ]
    default:
      return []
  }
}
export function gatewayImportURL(config: GatewayConfig, key: string): string {
  if (config.client === 'cc-switch') {
    const params = new URLSearchParams({
      resource: 'provider',
      app: config.target,
      name: config.name,
      homepage: config.origin,
      endpoint: gatewayEndpoint(config),
      apiKey: key,
      enabled: 'true',
    })
    if (config.model.trim()) params.set('model', config.model.trim())
    return `ccswitch://v1/import?${params}`
  }
  if (config.client === 'cherry-studio') {
    const value = JSON.stringify({
      id: 'gpt-load',
      name: config.name,
      type: 'openai',
      baseUrl: gatewayEndpoint(config),
      apiKey: key,
    })
    const bytes = new TextEncoder().encode(value)
    let binary = ''
    for (const byte of bytes) binary += String.fromCharCode(byte)
    const data = btoa(binary).replaceAll('+', '-').replaceAll('/', '_')
    return `cherrystudio://providers/api-keys?${new URLSearchParams({ v: '1', data })}`
  }
  throw new Error('UNSUPPORTED_GATEWAY_IMPORT')
}

// These clients discover available models themselves, so their config omits model names.
const modelFreeClients: readonly string[] = [
  'gemini-cli',
  'new-api',
  'nextchat',
  'open-webui',
  'cherry-studio',
]
export function gatewayNeedsModel(client: GatewayClientID, target?: GatewayTargetID): boolean {
  if (client === 'cc-switch' && target)
    return Boolean(gatewayTargets.find((entry) => entry.id === target)?.requiresModel)
  return !modelFreeClients.includes(client)
}

export const gatewaySlots = ['endpoint', 'apiKey', 'model'] as const
export type GatewaySlot = (typeof gatewaySlots)[number]
export interface GatewayField {
  slot: GatewaySlot
  value: string
}
// GUI clients need individual fields. Include the model only when the generated
// client config requires one, so the selector and output stay in sync.
export function gatewayFields(config: GatewayConfig, key: string): GatewayField[] {
  const model = config.model.trim()
  return [
    { slot: 'endpoint' as const, value: gatewayEndpoint(config) },
    { slot: 'apiKey' as const, value: key },
    ...(gatewayNeedsModel(config.client, config.target)
      ? [{ slot: 'model' as const, value: model }]
      : []),
  ]
}
