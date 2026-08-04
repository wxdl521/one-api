import { MiniAppApiError, request } from '../../lib/api'

export interface MiniAppTokenSummary {
  id: number
  name: string
  keyHint: string
  status: 1 | 2 | 3 | 4
  createdAt: number
  accessedAt: number
  expiresAt: number
  group: string
  modelLimits: string[]
}

export interface MiniAppCreatedToken {
  token: MiniAppTokenSummary
  tokenKey: string
}

export interface MiniAppTokenCreateInput {
  name: string
  group: string
  models: string[]
  expiresInDays: 7 | 30 | 90
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object'
}

function invalidTokenResponse(): MiniAppApiError {
  return new MiniAppApiError('MINIAPP_INVALID_TOKEN_RESPONSE', 'Mini Program token response is invalid')
}

function readTokenSummary(value: unknown): MiniAppTokenSummary {
  if (
    !isRecord(value) ||
    typeof value.id !== 'number' ||
    !Number.isInteger(value.id) ||
    value.id <= 0 ||
    typeof value.name !== 'string' ||
    typeof value.key_hint !== 'string' ||
    (value.status !== 1 && value.status !== 2 && value.status !== 3 && value.status !== 4) ||
    typeof value.created_at !== 'number' ||
    !Number.isFinite(value.created_at) ||
    typeof value.accessed_at !== 'number' ||
    !Number.isFinite(value.accessed_at) ||
    typeof value.expires_at !== 'number' ||
    !Number.isFinite(value.expires_at) ||
    typeof value.group !== 'string' ||
    !Array.isArray(value.model_limits) ||
    !value.model_limits.every((model) => typeof model === 'string')
  ) {
    throw invalidTokenResponse()
  }
  return {
    id: value.id,
    name: value.name,
    keyHint: value.key_hint,
    status: value.status,
    createdAt: value.created_at,
    accessedAt: value.accessed_at,
    expiresAt: value.expires_at,
    group: value.group,
    modelLimits: value.model_limits,
  }
}

export async function getMiniAppTokens(): Promise<MiniAppTokenSummary[]> {
  const response = await request<unknown>({ path: '/tokens', method: 'GET', auth: 'session' })
  if (!isRecord(response) || !Array.isArray(response.items)) {
    throw invalidTokenResponse()
  }
  return response.items.map(readTokenSummary)
}

export async function createMiniAppToken(input: MiniAppTokenCreateInput): Promise<MiniAppCreatedToken> {
  const response = await request<unknown, {
    name: string
    group: string
    models: string[]
    expires_in_days: number
  }>({
    path: '/tokens',
    method: 'POST',
    data: {
      name: input.name.trim(),
      group: input.group.trim(),
      models: input.models.map((model) => model.trim()),
      expires_in_days: input.expiresInDays,
    },
    auth: 'session',
  })
  if (!isRecord(response) || typeof response.token_key !== 'string' || response.token_key.trim() === '') {
    throw invalidTokenResponse()
  }
  return {
    token: readTokenSummary(response.token),
    tokenKey: response.token_key,
  }
}

export async function updateMiniAppTokenStatus(id: number, status: 1 | 2): Promise<MiniAppTokenSummary> {
  const response = await request<unknown, { status: 1 | 2 }>({
    path: `/tokens/${id}/status`,
    method: 'PATCH',
    data: { status },
    auth: 'session',
  })
  return readTokenSummary(response)
}

export async function revokeMiniAppToken(id: number): Promise<void> {
  await request<unknown>({
    path: `/tokens/${id}`,
    method: 'DELETE',
    auth: 'session',
  })
}
