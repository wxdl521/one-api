import Taro from '@tarojs/taro'

import { clearMiniAppSession, getMiniAppSession } from './session'

type ApiMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

type RequestAuth = 'session' | { bearerToken: string }

interface ApiEnvelope<T> {
  success: boolean
  data?: T
  message?: string
  code?: string
}

export interface ApiRequestOptions<TData = unknown> {
  path: string
  method: ApiMethod
  data?: TData
  auth?: RequestAuth
}

export class MiniAppApiError extends Error {
  readonly code: string
  readonly status: number
  readonly requestId?: string

  constructor(code: string, message: string, status = 0, requestId?: string) {
    super(message)
    this.name = 'MiniAppApiError'
    this.code = code
    this.status = status
    this.requestId = requestId
  }
}

const requestTimeout = 10_000
const miniAppApiPath = '/api/miniapp/v1'

function getBaseUrl(): string {
  const baseUrl = typeof __MINIAPP_API_BASE_URL__ === 'string'
    ? __MINIAPP_API_BASE_URL__.trim()
    : ''
  if (baseUrl === '') {
    throw new MiniAppApiError('MINIAPP_CONFIG_ERROR', 'Mini Program API is not configured')
  }

  let parsed: URL
  try {
    parsed = new URL(baseUrl)
  } catch {
    throw new MiniAppApiError('MINIAPP_CONFIG_ERROR', 'Mini Program API URL is invalid')
  }
  if (parsed.protocol !== 'https:' || parsed.hostname === '' || parsed.search !== '' || parsed.hash !== '') {
    throw new MiniAppApiError('MINIAPP_CONFIG_ERROR', 'Mini Program API URL must be an HTTPS origin')
  }
  return parsed.toString().replace(/\/$/, '')
}

function getRequestUrl(path: string): string {
  if (!path.startsWith('/')) {
    throw new MiniAppApiError('MINIAPP_CONFIG_ERROR', 'Mini Program API path is invalid')
  }
  return `${getBaseUrl()}${miniAppApiPath}${path}`
}

function getRequestId(): string {
  return `miniapp-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

function getHeaderValue(headers: unknown, name: string): string | undefined {
  if (headers === null || typeof headers !== 'object') {
    return undefined
  }
  for (const [key, value] of Object.entries(headers)) {
    if (key.toLowerCase() === name.toLowerCase() && typeof value === 'string') {
      return value
    }
  }
  return undefined
}

function sessionError(code: string, status: number): boolean {
  return status === 401 || code === 'MINIAPP_SESSION_INVALID' || code === 'AUTH_UNAUTHORIZED'
}

function toApiError(
  status: number,
  envelope: ApiEnvelope<unknown> | null,
  requestId?: string,
): MiniAppApiError {
  if (envelope === null) {
    return new MiniAppApiError('MINIAPP_INVALID_RESPONSE', 'Mini Program API returned an invalid response', status, requestId)
  }
  return new MiniAppApiError(
    envelope.code ?? 'MINIAPP_REQUEST_FAILED',
    envelope.message?.trim() || 'Mini Program request failed',
    status,
    requestId,
  )
}

function asEnvelope(value: unknown): ApiEnvelope<unknown> | null {
  if (value === null || typeof value !== 'object' || typeof (value as { success?: unknown }).success !== 'boolean') {
    return null
  }
  return value as ApiEnvelope<unknown>
}

export async function request<TResponse, TData = unknown>(
  options: ApiRequestOptions<TData>,
): Promise<TResponse> {
  const url = getRequestUrl(options.path)
  const requestId = getRequestId()
  const header: Record<string, string> = {
    'content-type': 'application/json',
    'X-Request-ID': requestId,
  }

  if (options.auth === 'session') {
    const session = getMiniAppSession()
    if (session === null) {
      throw new MiniAppApiError('MINIAPP_SESSION_UNAVAILABLE', 'Mini Program session is unavailable')
    }
    header.Authorization = `Bearer ${session.accessToken}`
  } else if (options.auth !== undefined) {
    const token = options.auth.bearerToken.trim()
    if (token === '') {
      throw new MiniAppApiError('MINIAPP_AUTH_UNAVAILABLE', 'Mini Program authorization is unavailable')
    }
    header.Authorization = `Bearer ${token}`
  }

  const attemptLimit = options.method === 'GET' ? 2 : 1
  let lastError: MiniAppApiError | null = null
  for (let attempt = 0; attempt < attemptLimit; attempt += 1) {
    try {
      const response = await Taro.request<ApiEnvelope<TResponse>>({
        url,
        method: options.method,
        data: options.data,
        header,
        timeout: requestTimeout,
      })
      const responseRequestId = getHeaderValue(response.header, 'x-request-id') ?? requestId
      const envelope = asEnvelope(response.data)
      if (response.statusCode >= 500 && options.method === 'GET' && attempt + 1 < attemptLimit) {
        lastError = toApiError(response.statusCode, envelope, responseRequestId)
        continue
      }
      if (response.statusCode < 200 || response.statusCode >= 300 || envelope === null || !envelope.success) {
        const error = toApiError(response.statusCode, envelope, responseRequestId)
        if (options.auth === 'session' && sessionError(error.code, error.status)) {
          clearMiniAppSession()
        }
        throw error
      }
      return envelope.data as TResponse
    } catch (error) {
      if (error instanceof MiniAppApiError) {
        throw error
      }
      lastError = new MiniAppApiError(
        'MINIAPP_NETWORK_ERROR',
        'Mini Program network request failed',
        0,
        requestId,
      )
      if (options.method !== 'GET' || attempt + 1 >= attemptLimit) {
        throw lastError
      }
    }
  }
  throw lastError ?? new MiniAppApiError('MINIAPP_NETWORK_ERROR', 'Mini Program network request failed', 0, requestId)
}
