import Taro from '@tarojs/taro'

import { request } from '../../lib/api'
import { clearMiniAppSession, getMiniAppSession, setMiniAppSession } from '../../lib/session'

interface BindingStatusResponse {
  status: 'pending' | 'bound' | 'expired'
}

export class MiniAppAuthError extends Error {
  readonly code: string

  constructor(code: string, message: string) {
    super(message)
    this.name = 'MiniAppAuthError'
    this.code = code
  }
}

let pendingIdentityTicket: string | null = null

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object'
}

function invalidAuthResponse(): MiniAppAuthError {
  return new MiniAppAuthError('MINIAPP_INVALID_AUTH_RESPONSE', 'Mini Program authentication response is invalid')
}

function storeSession(bundle: unknown): void {
  if (!isRecord(bundle) || !isRecord(bundle.session)) {
    throw invalidAuthResponse()
  }
  const accessToken = bundle.access_token
  const accessExpiresAt = bundle.access_expires_at
  const sid = bundle.session.sid
  if (
    typeof accessToken !== 'string' ||
    accessToken.trim() === '' ||
    typeof sid !== 'string' ||
    sid.trim() === '' ||
    typeof accessExpiresAt !== 'number' ||
    !Number.isFinite(accessExpiresAt) ||
    accessExpiresAt <= Math.floor(Date.now() / 1000)
  ) {
    throw invalidAuthResponse()
  }
  setMiniAppSession({
    accessToken,
    accessExpiresAt,
    sid,
  })
}

async function getWechatCode(): Promise<string> {
  try {
    const result = await Taro.login()
    const code = typeof result.code === 'string' ? result.code.trim() : ''
    if (code === '') {
      throw new MiniAppAuthError('MINIAPP_WECHAT_LOGIN_UNAVAILABLE', 'WeChat login code is unavailable')
    }
    return code
  } catch (error) {
    if (error instanceof MiniAppAuthError) {
      throw error
    }
    throw new MiniAppAuthError('MINIAPP_WECHAT_LOGIN_UNAVAILABLE', 'WeChat login is unavailable')
  }
}

function requirePendingTicket(): string {
  if (pendingIdentityTicket === null) {
    throw new MiniAppAuthError('MINIAPP_PENDING_IDENTITY_UNAVAILABLE', 'Mini Program identity verification has expired')
  }
  return pendingIdentityTicket
}

function getTrustedBindingOrigin(): URL {
  const rawOrigin = typeof __MINIAPP_BINDING_ORIGIN__ === 'string'
    ? __MINIAPP_BINDING_ORIGIN__.trim()
    : ''
  if (rawOrigin === '') {
    throw new MiniAppAuthError('MINIAPP_BINDING_CONFIGURATION_ERROR', 'Mini Program binding origin is unavailable')
  }
  try {
    const origin = new URL(rawOrigin)
    if (
      origin.protocol !== 'https:' ||
      origin.hostname === '' ||
      origin.username !== '' ||
      origin.password !== '' ||
      origin.pathname !== '/' ||
      origin.search !== '' ||
      origin.hash !== ''
    ) {
      throw new Error('invalid binding origin')
    }
    return origin
  } catch {
    throw new MiniAppAuthError('MINIAPP_BINDING_CONFIGURATION_ERROR', 'Mini Program binding origin is unavailable')
  }
}

export function getPendingIdentityTicket(): string | null {
  return pendingIdentityTicket
}

export function setPendingIdentityTicket(ticket: string): void {
  const normalizedTicket = ticket.trim()
  pendingIdentityTicket = normalizedTicket === '' ? null : normalizedTicket
}

export function clearPendingIdentityTicket(): void {
  pendingIdentityTicket = null
}

export async function loginWithWechat(): Promise<{ kind: 'authenticated' | 'pending' }> {
  const code = await getWechatCode()
  const result = await request<unknown>({
    path: '/auth/wechat',
    method: 'POST',
    data: { code },
  })
  if (!isRecord(result) || typeof result.state !== 'string') {
    throw invalidAuthResponse()
  }
  if (result.state === 'authenticated') {
    storeSession(result.session)
    clearPendingIdentityTicket()
    return { kind: 'authenticated' }
  }
  if (result.state === 'pending' && typeof result.pending_ticket === 'string' && result.pending_ticket.trim() !== '') {
    setPendingIdentityTicket(result.pending_ticket)
    clearMiniAppSession()
    return { kind: 'pending' }
  }
  throw invalidAuthResponse()
}

export async function renewMiniAppSession(): Promise<void> {
  const session = getMiniAppSession()
  if (session === null) {
    throw new MiniAppAuthError('MINIAPP_SESSION_UNAVAILABLE', 'Mini Program session is unavailable')
  }
  const code = await getWechatCode()
  const bundle = await request<unknown>({
    path: '/auth/renew',
    method: 'POST',
    data: { code, sid: session.sid },
  })
  storeSession(bundle)
}

export async function registerWithPendingIdentity(username: string, password: string): Promise<void> {
  const bundle = await request<unknown>({
    path: '/auth/register',
    method: 'POST',
    data: {
      pending_identity_ticket: requirePendingTicket(),
      username: username.trim(),
      password,
    },
  })
  storeSession(bundle)
  clearPendingIdentityTicket()
}

export async function createBinding(): Promise<{ bindingId: string; webUrl: string }> {
  const result = await request<unknown>({
    path: '/bindings',
    method: 'POST',
    data: { pending_identity_ticket: requirePendingTicket() },
  })
  if (!isRecord(result)) {
    throw new MiniAppAuthError('MINIAPP_INVALID_BINDING_RESPONSE', 'Mini Program binding response is invalid')
  }
  const bindingId = typeof result.binding_id === 'string' ? result.binding_id.trim() : ''
  const webUrl = typeof result.web_url === 'string'
    ? result.web_url.trim()
    : typeof result.bind_url === 'string'
      ? result.bind_url.trim()
      : ''
  if (bindingId === '' || webUrl === '') {
    throw new MiniAppAuthError('MINIAPP_INVALID_BINDING_RESPONSE', 'Mini Program binding response is invalid')
  }
  try {
    const trustedOrigin = getTrustedBindingOrigin()
    const parsedUrl = new URL(webUrl)
    if (
      parsedUrl.origin !== trustedOrigin.origin ||
      parsedUrl.username !== '' ||
      parsedUrl.password !== '' ||
      parsedUrl.pathname !== '/miniapp-bind' ||
      parsedUrl.search !== ''
    ) {
      throw new Error('invalid web URL')
    }
  } catch (error) {
    if (error instanceof MiniAppAuthError) {
      throw error
    }
    throw new MiniAppAuthError('MINIAPP_INVALID_BINDING_RESPONSE', 'Mini Program binding response is invalid')
  }
  return { bindingId, webUrl }
}

export async function getBindingStatus(bindingId: string): Promise<BindingStatusResponse['status']> {
  const result = await request<unknown>({
    path: `/bindings/${encodeURIComponent(bindingId)}`,
    method: 'GET',
    auth: { bearerToken: requirePendingTicket() },
  })
  if (!isRecord(result) || (result.status !== 'pending' && result.status !== 'bound' && result.status !== 'expired')) {
    throw new MiniAppAuthError('MINIAPP_INVALID_BINDING_RESPONSE', 'Mini Program binding response is invalid')
  }
  return result.status
}
