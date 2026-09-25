import type { ApiClient } from '@shared/http/client'
import { InvalidResponseError } from '@shared/http/errors'
import type { AuthSessionPayload } from '@shared/http/types'

export async function getAuthSession(
  client: ApiClient,
  credential: string,
  signal: AbortSignal,
  handleUnauthorized = false,
): Promise<AuthSessionPayload> {
  const data = await client.request<unknown>('/api/auth/session', {
    authKey: credential,
    cache: 'no-store',
    signal,

    handleUnauthorized,
  })
  if (
    typeof data !== 'object' ||
    data === null ||
    !('authenticated' in data) ||
    data.authenticated !== true ||
    !('principal_type' in data) ||
    (data.principal_type !== 'admin' && data.principal_type !== 'access_key')
  ) {
    throw new InvalidResponseError()
  }
  return { authenticated: true, principal_type: data.principal_type }
}
