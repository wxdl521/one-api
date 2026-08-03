import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const taro = vi.hoisted(() => ({
  request: vi.fn(),
}))

vi.mock('@tarojs/taro', () => ({ default: taro, ...taro }))

const apiBase = globalThis as typeof globalThis & {
  __MINIAPP_API_BASE_URL__?: string
}

async function loadApi(baseUrl = 'https://gateway.example.com') {
  apiBase.__MINIAPP_API_BASE_URL__ = baseUrl
  return import('./api')
}

describe('mini program API client', () => {
  beforeEach(() => {
    vi.resetModules()
    taro.request.mockReset()
  })

  afterEach(() => {
    delete apiBase.__MINIAPP_API_BASE_URL__
  })

  it('attaches the in-memory session bearer only for a protected request', async () => {
    const { request } = await loadApi()
    const { setMiniAppSession } = await import('./session')
    setMiniAppSession({
      accessToken: 'miniapp-access-token',
      accessExpiresAt: Math.floor(Date.now() / 1000) + 60,
      sid: 'miniapp-session-id',
    })
    taro.request.mockResolvedValue({
      statusCode: 200,
      header: { 'x-request-id': 'server-request-id' },
      data: { success: true, message: '', data: { ok: true } },
    })

    await expect(
      request<{ ok: boolean }>({ path: '/auth/logout', method: 'POST', auth: 'session' }),
    ).resolves.toEqual({ ok: true })

    expect(taro.request).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://gateway.example.com/api/miniapp/v1/auth/logout',
        method: 'POST',
        timeout: 10_000,
        header: expect.objectContaining({
          Authorization: 'Bearer miniapp-access-token',
          'X-Request-ID': expect.any(String),
        }),
      }),
    )
  })

  it('does not add an authorization header to anonymous requests', async () => {
    const { request } = await loadApi()
    taro.request.mockResolvedValue({
      statusCode: 200,
      header: {},
      data: { success: true, message: '', data: { state: 'pending' } },
    })

    await request({ path: '/auth/wechat', method: 'POST', data: { code: 'wechat-code' } })

    const options = taro.request.mock.calls[0]?.[0] as { header: Record<string, string> }
    expect(options.header.Authorization).toBeUndefined()
  })

  it('retries a transient GET once but never retries a POST', async () => {
    const { request } = await loadApi()
    taro.request
      .mockRejectedValueOnce(new Error('network unavailable'))
      .mockResolvedValueOnce({
        statusCode: 200,
        header: {},
        data: { success: true, message: '', data: { status: 'pending' } },
      })

    await expect(request({ path: '/bindings/binding-id', method: 'GET' })).resolves.toEqual({
      status: 'pending',
    })
    expect(taro.request).toHaveBeenCalledTimes(2)

    taro.request.mockReset().mockRejectedValue(new Error('network unavailable'))
    await expect(request({ path: '/auth/wechat', method: 'POST' })).rejects.toMatchObject({
      code: 'MINIAPP_NETWORK_ERROR',
    })
    expect(taro.request).toHaveBeenCalledTimes(1)
  })

  it('clears a rejected session and preserves the server request ID on the error', async () => {
    const { request } = await loadApi()
    const { getMiniAppSession, setMiniAppSession } = await import('./session')
    setMiniAppSession({
      accessToken: 'rejected-access-token',
      accessExpiresAt: Math.floor(Date.now() / 1000) + 60,
      sid: 'rejected-session-id',
    })
    taro.request.mockResolvedValue({
      statusCode: 401,
      header: { 'x-request-id': 'server-request-id' },
      data: { success: false, code: 'MINIAPP_SESSION_INVALID', message: 'Unauthorized' },
    })

    await expect(
      request({ path: '/auth/logout', method: 'POST', auth: 'session' }),
    ).rejects.toMatchObject({
      code: 'MINIAPP_SESSION_INVALID',
      requestId: 'server-request-id',
    })
    expect(getMiniAppSession()).toBeNull()
  })

  it('fails closed when the compiled API base URL is absent', async () => {
    const { request } = await loadApi('')

    await expect(request({ path: '/auth/wechat', method: 'POST' })).rejects.toMatchObject({
      code: 'MINIAPP_CONFIG_ERROR',
    })
    expect(taro.request).not.toHaveBeenCalled()
  })
})
