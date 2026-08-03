import Taro from '@tarojs/taro'

import { request } from '../../lib/api'
import { clearMiniAppSession, getMiniAppSession, setMiniAppSession } from '../../lib/session'

interface AuthBundleResponse {
  access_token: string
  access_expires_at: number
  session: {
    sid: string
  }
}

interface WechatLoginResponse {
  state: 'authenticated' | 'pending'
  pending_ticket?: string
  session?: AuthBundleResponse
}

interface BindingStartResponse {
  binding_id: string
  web_url?: string
  bind_url?: string
}

interface BindingStatusResponse {
  id: string
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

function storeSession(bundle: AuthBundleResponse): void {
  const accessToken = bundle.access_token?.trim()
  const sid = bundle.session?.sid?.trim()
  if (accessToken === '' || sid === '' || !Number.isFinite(bundle.access_expires_at)) {
    throw new MiniAppAuthError('MINIAPP_INVALID_AUTH_RESPONSE', 'Mini Program authentication response is invalid')
  }
  setMiniAppSession({
    accessToken,
    accessExpiresAt: bundle.access_expires_at,
    sid,
  })
}

async function getWechatCode(): Promise<string> {
  try {
    const result = await Taro.login()
    const code = result.code?.trim()
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
  const result = await request<WechatLoginResponse>({
    path: '/auth/wechat',
    method: 'POST',
    data: { code },
  })
  if (result.state === 'authenticated' && result.session !== undefined) {
    storeSession(result.session)
    clearPendingIdentityTicket()
    return { kind: 'authenticated' }
  }
  if (result.state === 'pending' && result.pending_ticket?.trim()) {
    setPendingIdentityTicket(result.pending_ticket)
    clearMiniAppSession()
    return { kind: 'pending' }
  }
  throw new MiniAppAuthError('MINIAPP_INVALID_AUTH_RESPONSE', 'Mini Program authentication response is invalid')
}

export async function renewMiniAppSession(): Promise<void> {
  const session = getMiniAppSession()
  if (session === null) {
    throw new MiniAppAuthError('MINIAPP_SESSION_UNAVAILABLE', 'Mini Program session is unavailable')
  }
  const code = await getWechatCode()
  const bundle = await request<AuthBundleResponse>({
    path: '/auth/renew',
    method: 'POST',
    data: { code, sid: session.sid },
  })
  storeSession(bundle)
}

export async function registerWithPendingIdentity(username: string, password: string): Promise<void> {
  const bundle = await request<AuthBundleResponse>({
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
  const result = await request<BindingStartResponse>({
    path: '/bindings',
    method: 'POST',
    data: { pending_identity_ticket: requirePendingTicket() },
  })
  const bindingId = result.binding_id?.trim()
  const webUrl = (result.web_url ?? result.bind_url ?? '').trim()
  if (bindingId === '' || webUrl === '') {
    throw new MiniAppAuthError('MINIAPP_INVALID_BINDING_RESPONSE', 'Mini Program binding response is invalid')
  }
  try {
    const parsedUrl = new URL(webUrl)
    if (parsedUrl.protocol !== 'https:' || parsedUrl.hostname === '') {
      throw new Error('invalid web URL')
    }
  } catch {
    throw new MiniAppAuthError('MINIAPP_INVALID_BINDING_RESPONSE', 'Mini Program binding response is invalid')
  }
  return { bindingId, webUrl }
}

export async function getBindingStatus(bindingId: string): Promise<BindingStatusResponse['status']> {
  const result = await request<BindingStatusResponse>({
    path: `/bindings/${encodeURIComponent(bindingId)}`,
    method: 'GET',
    auth: { bearerToken: requirePendingTicket() },
  })
  if (result.status !== 'pending' && result.status !== 'bound' && result.status !== 'expired') {
    throw new MiniAppAuthError('MINIAPP_INVALID_BINDING_RESPONSE', 'Mini Program binding response is invalid')
  }
  return result.status
}
