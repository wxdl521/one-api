import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const taro = vi.hoisted(() => ({
  getStorageSync: vi.fn(),
  removeStorageSync: vi.fn(),
  setStorageSync: vi.fn(),
}))

vi.mock('@tarojs/taro', () => ({ default: taro, ...taro }))

describe('pending mini text-test request storage', () => {
  beforeEach(() => {
    vi.resetModules()
    taro.getStorageSync.mockReset()
    taro.removeStorageSync.mockReset()
    taro.setStorageSync.mockReset()
  })

  afterEach(async () => {
    const { clearPendingTextTestRequestID } = await import('./pending-text-test')
    clearPendingTextTestRequestID()
  })

  it('persists only one valid request UUID without prompt or output data', async () => {
    const { getPendingTextTestRequestID, setPendingTextTestRequestID } = await import('./pending-text-test')
    const requestID = '00010203-0405-4607-8809-0a0b0c0d0e0f'

    setPendingTextTestRequestID(requestID)
    taro.getStorageSync.mockReturnValue(requestID)

    expect(taro.setStorageSync).toHaveBeenCalledWith('miniapp.pending-text-test-request-id.v1', requestID)
    expect(getPendingTextTestRequestID()).toBe(requestID)
    expect(JSON.stringify(taro.setStorageSync.mock.calls)).not.toContain('prompt')
    expect(JSON.stringify(taro.setStorageSync.mock.calls)).not.toContain('output')
  })

  it('clears malformed persisted data instead of treating it as a status request', async () => {
    const { getPendingTextTestRequestID } = await import('./pending-text-test')
    taro.getStorageSync.mockReturnValue({ requestID: 'not-a-uuid', input: 'must not persist' })

    expect(getPendingTextTestRequestID()).toBeNull()
    expect(taro.removeStorageSync).toHaveBeenCalledWith('miniapp.pending-text-test-request-id.v1')
  })
})
