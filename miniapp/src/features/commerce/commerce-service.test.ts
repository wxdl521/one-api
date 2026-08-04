import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const taro = vi.hoisted(() => ({
  request: vi.fn(),
}))

vi.mock('@tarojs/taro', () => ({ default: taro, ...taro }))

const apiBase = globalThis as typeof globalThis & {
  __MINIAPP_API_BASE_URL__?: string
  __MINIAPP_BINDING_ORIGIN__?: string
}

describe('mini program commerce service', () => {
  beforeEach(() => {
    vi.resetModules()
    taro.request.mockReset()
    apiBase.__MINIAPP_API_BASE_URL__ = 'https://console.example.com'
    apiBase.__MINIAPP_BINDING_ORIGIN__ = 'https://console.example.com'
  })

  afterEach(async () => {
    const { clearMiniAppSession } = await import('../../lib/session')
    clearMiniAppSession()
    delete apiBase.__MINIAPP_API_BASE_URL__
    delete apiBase.__MINIAPP_BINDING_ORIGIN__
  })

  it('loads the read-only plans, products, and orders through the Mini BFF', async () => {
    const { setMiniAppSession } = await import('../../lib/session')
    setMiniAppSession({
      accessToken: 'miniapp-access-token',
      accessExpiresAt: Math.floor(Date.now() / 1000) + 60,
      sid: 'miniapp-session-id',
    })
    taro.request
      .mockResolvedValueOnce({
        statusCode: 200,
        header: {},
        data: { success: true, data: [{ id: 1, title: 'Starter', subtitle: '', price_amount: 12.5, currency: 'USD', duration_unit: 'month', duration_value: 1 }] },
      })
      .mockResolvedValueOnce({
        statusCode: 200,
        header: {},
        data: { success: true, data: [{ id: 2, name: 'Consulting', summary: '', description: '', image_url: '', price_cents: 2500, product_type: 'manual' }] },
      })
      .mockResolvedValueOnce({
        statusCode: 200,
        header: {},
        data: { success: true, data: [{ id: 3, product_name: 'Consulting', price_cents: 2500, payment_status: 'pending', fulfillment_status: 'pending', created_at: 1_800_000_000 }] },
      })

    const { getMiniAppCommerce } = await import('./commerce-service')

    await expect(getMiniAppCommerce()).resolves.toEqual({
      plans: [{ id: 1, title: 'Starter', subtitle: '', priceAmount: 12.5, currency: 'USD', durationUnit: 'month', durationValue: 1 }],
      products: [{ id: 2, name: 'Consulting', summary: '', description: '', imageUrl: '', priceCents: 2500, productType: 'manual' }],
      orders: [{ id: 3, productName: 'Consulting', priceCents: 2500, paymentStatus: 'pending', fulfillmentStatus: 'pending', createdAt: 1_800_000_000 }],
    })
    expect(taro.request).toHaveBeenCalledTimes(3)
    expect(taro.request.mock.calls.map(([options]) => options.method)).toEqual(['GET', 'GET', 'GET'])
    expect(taro.request.mock.calls.map(([options]) => options.url)).toEqual([
      'https://console.example.com/api/miniapp/v1/plans',
      'https://console.example.com/api/miniapp/v1/products',
      'https://console.example.com/api/miniapp/v1/orders',
    ])
  })

  it('accepts only the configured business checkout URL with one fragment ticket', async () => {
    const { setMiniAppSession } = await import('../../lib/session')
    setMiniAppSession({
      accessToken: 'miniapp-access-token',
      accessExpiresAt: Math.floor(Date.now() / 1000) + 60,
      sid: 'miniapp-session-id',
    })
    taro.request.mockResolvedValue({
      statusCode: 200,
      header: {},
      data: { success: true, data: { checkout_url: 'https://console.example.com/miniapp-checkout#checkout_ticket=opaque-ticket' } },
    })

    const { startMiniAppCheckout } = await import('./commerce-service')

    await expect(startMiniAppCheckout('product', 2)).resolves.toBe(
      'https://console.example.com/miniapp-checkout#checkout_ticket=opaque-ticket',
    )
    expect(taro.request).toHaveBeenCalledWith(expect.objectContaining({
      method: 'POST',
      url: 'https://console.example.com/api/miniapp/v1/checkout',
      data: { target_type: 'product', target_id: 2 },
    }))
  })
})
