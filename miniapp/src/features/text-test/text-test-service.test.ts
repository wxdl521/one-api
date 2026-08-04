import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const taro = vi.hoisted(() => ({
  getRandomValues: vi.fn(),
  request: vi.fn(),
}))

vi.mock('@tarojs/taro', () => ({ default: taro, ...taro }))

const apiBase = globalThis as typeof globalThis & {
  __MINIAPP_API_BASE_URL__?: string
}

describe('mini program text-test service', () => {
  beforeEach(() => {
    vi.resetModules()
    taro.getRandomValues.mockReset()
    taro.request.mockReset()
    apiBase.__MINIAPP_API_BASE_URL__ = 'https://gateway.example.com'
  })

  afterEach(async () => {
    const { clearMiniAppSession } = await import('../../lib/session')
    clearMiniAppSession()
    delete apiBase.__MINIAPP_API_BASE_URL__
  })

  it('creates a cryptographically random version-four UUID for an idempotent text-test attempt', async () => {
    taro.getRandomValues.mockResolvedValue({
      randomValues: Uint8Array.from([0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15]).buffer,
    })
    const { createMiniTextTestRequestID } = await import('./text-test-service')

    await expect(createMiniTextTestRequestID()).resolves.toBe('00010203-0405-4607-8809-0a0b0c0d0e0f')
    expect(taro.getRandomValues).toHaveBeenCalledWith({ length: 16 })
  })

  it('accepts at most 4,000 Unicode code points before a request can be submitted', async () => {
    const { isMiniTextTestInputValid } = await import('./text-test-service')

    expect(isMiniTextTestInputValid('你'.repeat(4_000))).toBe(true)
    expect(isMiniTextTestInputValid('🙂'.repeat(4_000))).toBe(true)
    expect(isMiniTextTestInputValid('你'.repeat(4_001))).toBe(false)
    expect(isMiniTextTestInputValid('   ')).toBe(false)
  })

  it('uses only the restricted Mini BFF request shape and projects no prompt or output fields', async () => {
    const { setMiniAppSession } = await import('../../lib/session')
    setMiniAppSession({
      accessToken: 'miniapp-access-token',
      accessExpiresAt: Math.floor(Date.now() / 1000) + 60,
      sid: 'miniapp-session-id',
    })
    taro.request.mockResolvedValue({
      statusCode: 200,
      header: {},
      data: {
        success: true,
        data: {
          request_id: '00010203-0405-4607-8809-0a0b0c0d0e0f',
          model: 'gpt-mini',
          state: 'succeeded',
          charge_ref: 'server-request-id',
          charged_quota: 12,
          created_at: 1,
          started_at: 1,
          completed_at: 2,
          input: 'must not escape the request boundary',
          output: 'must not be returned by the miniapp service',
        },
      },
    })
    const { startMiniTextTest } = await import('./text-test-service')

    await expect(startMiniTextTest({
      clientRequestID: '00010203-0405-4607-8809-0a0b0c0d0e0f',
      model: 'gpt-mini',
      input: 'transient prompt',
    })).resolves.toEqual({
      requestID: '00010203-0405-4607-8809-0a0b0c0d0e0f',
      model: 'gpt-mini',
      state: 'succeeded',
      chargeReference: 'server-request-id',
      chargedQuota: 12,
      errorCode: '',
      createdAt: 1,
      startedAt: 1,
      completedAt: 2,
    })
    expect(taro.request).toHaveBeenCalledWith(expect.objectContaining({
      method: 'POST',
      url: 'https://gateway.example.com/api/miniapp/v1/text-tests',
      timeout: 20_000,
      data: {
        client_request_id: '00010203-0405-4607-8809-0a0b0c0d0e0f',
        model: 'gpt-mini',
        input: 'transient prompt',
      },
    }))
  })
})
