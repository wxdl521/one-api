import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const taro = vi.hoisted(() => ({
  login: vi.fn(),
  request: vi.fn(),
}))

vi.mock('@tarojs/taro', () => ({ default: taro, ...taro }))

const apiBase = globalThis as typeof globalThis & {
  __MINIAPP_API_BASE_URL__?: string
}

async function loadService() {
  apiBase.__MINIAPP_API_BASE_URL__ = 'https://gateway.example.com'
  return import('./auth-service')
}

describe('WeChat mini program authentication service', () => {
  beforeEach(() => {
    vi.resetModules()
    taro.login.mockReset()
    taro.request.mockReset()
  })

  afterEach(async () => {
    const { clearMiniAppSession } = await import('../../lib/session')
    clearMiniAppSession()
    delete apiBase.__MINIAPP_API_BASE_URL__
  })

  it('exchanges a wx.login code and keeps a normal session in memory', async () => {
    const { loginWithWechat } = await loadService()
    taro.login.mockResolvedValue({ code: 'wx-login-code' })
    taro.request.mockResolvedValue({
      statusCode: 200,
      header: {},
      data: {
        success: true,
        message: '',
        data: {
          state: 'authenticated',
          session: {
            access_token: 'miniapp-access-token',
            access_expires_at: Math.floor(Date.now() / 1000) + 60,
            session: { sid: 'miniapp-session-id' },
          },
        },
      },
    })

    await expect(loginWithWechat()).resolves.toEqual({ kind: 'authenticated' })
    expect(taro.request).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://gateway.example.com/api/miniapp/v1/auth/wechat',
        method: 'POST',
        data: { code: 'wx-login-code' },
      }),
    )

    const { getMiniAppSession } = await import('../../lib/session')
    expect(getMiniAppSession()).toMatchObject({
      accessToken: 'miniapp-access-token',
      sid: 'miniapp-session-id',
    })
  })

  it('holds an unbound identity ticket in memory for the binding flow', async () => {
    const { getPendingIdentityTicket, loginWithWechat } = await loadService()
    taro.login.mockResolvedValue({ code: 'wx-login-code' })
    taro.request.mockResolvedValue({
      statusCode: 200,
      header: {},
      data: {
        success: true,
        message: '',
        data: { state: 'pending', pending_ticket: 'opaque-pending-ticket' },
      },
    })

    await expect(loginWithWechat()).resolves.toEqual({ kind: 'pending' })
    expect(getPendingIdentityTicket()).toBe('opaque-pending-ticket')
    const { getMiniAppSession } = await import('../../lib/session')
    expect(getMiniAppSession()).toBeNull()
  })

  it('uses the pending ticket only where the binding API requires it', async () => {
    const { createBinding, getBindingStatus, setPendingIdentityTicket } = await loadService()
    setPendingIdentityTicket('opaque-pending-ticket')
    taro.request
      .mockResolvedValueOnce({
        statusCode: 200,
        header: {},
        data: {
          success: true,
          message: '',
          data: {
            binding_id: 'binding-id',
            web_url: 'https://console.example.com/miniapp-bind#binding_ticket=opaque-browser-ticket',
          },
        },
      })
      .mockResolvedValueOnce({
        statusCode: 200,
        header: {},
        data: { success: true, message: '', data: { id: 'binding-id', status: 'pending' } },
      })

    await expect(createBinding()).resolves.toEqual({
      bindingId: 'binding-id',
      webUrl: 'https://console.example.com/miniapp-bind#binding_ticket=opaque-browser-ticket',
    })
    await expect(getBindingStatus('binding-id')).resolves.toBe('pending')

    expect(taro.request.mock.calls[0]?.[0]).toMatchObject({
      method: 'POST',
      data: { pending_identity_ticket: 'opaque-pending-ticket' },
    })
    expect(taro.request.mock.calls[0]?.[0].header.Authorization).toBeUndefined()
    expect(taro.request.mock.calls[1]?.[0]).toMatchObject({
      method: 'GET',
      header: { Authorization: 'Bearer opaque-pending-ticket' },
    })
  })

  it('fails with a safe error when wx.login does not provide a code', async () => {
    const { loginWithWechat } = await loadService()
    taro.login.mockResolvedValue({ code: '' })

    await expect(loginWithWechat()).rejects.toMatchObject({
      code: 'MINIAPP_WECHAT_LOGIN_UNAVAILABLE',
    })
    expect(taro.request).not.toHaveBeenCalled()
  })
})
