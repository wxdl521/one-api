import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const taro = vi.hoisted(() => ({
  request: vi.fn(),
}))

vi.mock('@tarojs/taro', () => ({ default: taro, ...taro }))

const apiBase = globalThis as typeof globalThis & {
  __MINIAPP_API_BASE_URL__?: string
}

describe('mini program account service', () => {
  beforeEach(() => {
    vi.resetModules()
    taro.request.mockReset()
    apiBase.__MINIAPP_API_BASE_URL__ = 'https://gateway.example.com'
  })

  afterEach(async () => {
    const { clearMiniAppSession } = await import('../../lib/session')
    clearMiniAppSession()
    delete apiBase.__MINIAPP_API_BASE_URL__
  })

  it('loads the safe overview with the Mini Program session', async () => {
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
        message: '',
        data: {
          username: 'mini-user',
          display_name: 'Mini User',
          email: 'm***@example.com',
          quota: { balance: 1200, unit: 'quota' },
          enabled_groups: ['default'],
          subscriptions: [{
            plan_title: 'Monthly plan',
            status: 'active',
            ends_at: 1_800_000_000,
            quota: { remaining: 300, unlimited: false, unit: 'quota' },
          }],
        },
      },
    })

    const { getAccountOverview } = await import('./account-service')

    await expect(getAccountOverview()).resolves.toEqual({
      username: 'mini-user',
      displayName: 'Mini User',
      email: 'm***@example.com',
      quota: { balance: 1200, unit: 'quota' },
      enabledGroups: ['default'],
      subscriptions: [{
        planTitle: 'Monthly plan',
        status: 'active',
        endsAt: 1_800_000_000,
        quota: { remaining: 300, unlimited: false, unit: 'quota' },
      }],
    })
    expect(taro.request).toHaveBeenCalledWith(expect.objectContaining({
      method: 'GET',
      url: 'https://gateway.example.com/api/miniapp/v1/me/overview',
      header: expect.objectContaining({ Authorization: 'Bearer miniapp-access-token' }),
    }))
  })
})
