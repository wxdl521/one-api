import { describe, expect, it, vi } from 'vitest'

const taro = vi.hoisted(() => ({
  getStorageSync: vi.fn(),
  removeStorageSync: vi.fn(),
  setStorageSync: vi.fn(),
}))

vi.mock('@tarojs/taro', () => ({ default: taro, ...taro }))

import {
  TextTestLifecycle,
  type MiniTextTestStatus,
} from '../../features/text-test/text-test-lifecycle'

const runningStatus: MiniTextTestStatus = {
  requestID: 'test-request-id',
  model: 'gpt-mini',
  state: 'running',
  chargeReference: '',
  chargedQuota: 0,
  errorCode: '',
  createdAt: 1,
  startedAt: 1,
  completedAt: 0,
}

function createLifecycle(options?: {
  initialPendingRequestID?: string | null
  isRetryableError?: (error: unknown) => boolean
}) {
  const createRequestID = vi.fn().mockResolvedValue('test-request-id')
  const getStatus = vi.fn().mockResolvedValue(runningStatus)
  const onError = vi.fn()
  const onPending = vi.fn()
  const onStartChange = vi.fn()
  const onStatus = vi.fn()
  const persistPendingRequestID = vi.fn()
  const start = vi.fn().mockResolvedValue(runningStatus)
  const clearTimeout = vi.fn()
  const setTimeout = vi.fn<(callback: () => void, delay: number) => ReturnType<typeof globalThis.setTimeout>>(
    () => 1 as unknown as ReturnType<typeof globalThis.setTimeout>,
  )
  const now = vi.fn(() => 0)
  const lifecycle = new TextTestLifecycle({
    createRequestID,
    getStatus,
    getPersistedRequestID: () => options?.initialPendingRequestID ?? null,
    isRetryableError: options?.isRetryableError,
    onError,
    onPending,
    onStartChange,
    onStatus,
    start,
    clearTimeout,
    now,
    setTimeout,
    setPersistedRequestID: persistPendingRequestID,
  })
  return {
    lifecycle,
    createRequestID,
    getStatus,
    onError,
    onPending,
    onStartChange,
    onStatus,
    persistPendingRequestID,
    start,
    clearTimeout,
    setTimeout,
    now,
  }
}

describe('text-test page lifecycle', () => {
  it('keeps one request ID while an attempt is pending instead of starting another text test', async () => {
    const { lifecycle, createRequestID, getStatus, now, start } = createLifecycle()
    lifecycle.show()

    await lifecycle.submit({ model: 'gpt-mini', input: 'first transient prompt' })
    now.mockReturnValue(20_000)
    await lifecycle.submit({ model: 'gpt-mini', input: 'first transient prompt' })

    expect(createRequestID).toHaveBeenCalledTimes(1)
    expect(start).toHaveBeenCalledTimes(1)
    expect(getStatus).toHaveBeenCalledWith('test-request-id')
  })

  it('does not query status while the first start call is still in flight', async () => {
    let resolveStart: (status: MiniTextTestStatus) => void = () => undefined
    const pendingStart = new Promise<MiniTextTestStatus>((resolve) => {
      resolveStart = resolve
    })
    const { lifecycle, createRequestID, getStatus, start } = createLifecycle()
    start.mockReturnValue(pendingStart)
    lifecycle.show()

    const firstSubmit = lifecycle.submit({ model: 'gpt-mini', input: 'start before checking status' })
    await Promise.resolve()
    await lifecycle.submit({ model: 'gpt-mini', input: 'start before checking status' })

    expect(createRequestID).toHaveBeenCalledTimes(1)
    expect(start).toHaveBeenCalledTimes(1)
    expect(getStatus).not.toHaveBeenCalled()

    resolveStart({ ...runningStatus })
    await firstSubmit
  })

  it('waits for an in-flight start across hide, show, unload, and restore before checking status', async () => {
    let resolveStart: (status: MiniTextTestStatus) => void = () => undefined
    const pendingStart = new Promise<MiniTextTestStatus>((resolve) => {
      resolveStart = resolve
    })
    const first = createLifecycle()
    first.start.mockReturnValue(pendingStart)
    first.lifecycle.show()

    const firstSubmit = first.lifecycle.submit({ model: 'gpt-mini', input: 'wait for the post to settle' })
    await Promise.resolve()
    first.lifecycle.hide()
    first.lifecycle.show()

    expect(first.getStatus).not.toHaveBeenCalled()
    expect(first.onStartChange).toHaveBeenLastCalledWith(true)

    first.lifecycle.unload()
    const restored = createLifecycle({ initialPendingRequestID: 'test-request-id' })
    restored.lifecycle.show()

    expect(restored.getStatus).not.toHaveBeenCalled()
    const poll = restored.setTimeout.mock.calls[0]?.[0] as (() => void)

    resolveStart({ ...runningStatus })
    await firstSubmit
    await poll()

    expect(first.onStartChange).toHaveBeenLastCalledWith(false)
    expect(restored.getStatus).toHaveBeenCalledWith('test-request-id')
  })

  it('does not poll past the 20-second foreground budget and leaves the same attempt pending', async () => {
    const { lifecycle, getStatus, onPending, setTimeout, now } = createLifecycle()
    lifecycle.show()

    await lifecycle.submit({ model: 'gpt-mini', input: 'bounded polling' })
    const poll = setTimeout.mock.calls[0]?.[0] as (() => void)
    now.mockReturnValue(20_000)
    await poll()

    expect(getStatus).not.toHaveBeenCalled()
    expect(onPending).toHaveBeenLastCalledWith('test-request-id', 'textTestPending')
  })

  it('cancels polling and ignores late status results after the page is hidden or unloaded', async () => {
    let resolveStatus: (status: MiniTextTestStatus) => void = () => undefined
    const pendingStatus = new Promise<MiniTextTestStatus>((resolve) => {
      resolveStatus = resolve
    })
    const { lifecycle, onStatus, clearTimeout, getStatus } = createLifecycle()
    getStatus.mockReturnValue(pendingStatus)
    lifecycle.show()
    await lifecycle.submit({ model: 'gpt-mini', input: 'do not retain after leaving' })

    const check = lifecycle.checkStatus()
    lifecycle.hide()
    lifecycle.unload()
    resolveStatus({ ...runningStatus, state: 'succeeded' })
    await check

    expect(clearTimeout).toHaveBeenCalled()
    expect(onStatus).toHaveBeenCalledTimes(1)
    expect(lifecycle.getPendingRequestID()).toBe('test-request-id')
  })

  it('releases a request ID when the server rejects it before creating an attempt', async () => {
    const rejected = new Error('model is no longer available')
    const { lifecycle, onError, setTimeout, start } = createLifecycle({
      isRetryableError: () => false,
    })
    start.mockRejectedValue(rejected)
    lifecycle.show()

    await lifecycle.submit({ model: 'gpt-mini', input: 'new attempt can be made safely' })

    expect(onError).toHaveBeenCalledWith(rejected)
    expect(setTimeout).not.toHaveBeenCalled()
    expect(lifecycle.getPendingRequestID()).toBeNull()
  })

  it('retains the persisted ID after an unload and resumes status polling with it', async () => {
    const { lifecycle, getStatus, persistPendingRequestID } = createLifecycle()
    lifecycle.show()
    await lifecycle.submit({ model: 'gpt-mini', input: 'persist only the request id' })
    lifecycle.unload()

    expect(persistPendingRequestID).toHaveBeenCalledWith('test-request-id')
    expect(persistPendingRequestID).not.toHaveBeenCalledWith(null)

    const restored = createLifecycle({ initialPendingRequestID: 'test-request-id' })
    restored.lifecycle.show()
    await Promise.resolve()

    expect(restored.getStatus).toHaveBeenCalledWith('test-request-id')
    expect(getStatus).not.toHaveBeenCalled()
  })

  it('keeps a transitional not-found status on the same request ID for bounded retry', async () => {
    const notFound = new Error('attempt is not visible yet')
    const { lifecycle, onError, persistPendingRequestID, setTimeout, getStatus } = createLifecycle({
      isRetryableError: (error) => error === notFound,
    })
    getStatus.mockRejectedValue(notFound)
    lifecycle.show()
    await lifecycle.submit({ model: 'gpt-mini', input: 'await durable server attempt' })

    await lifecycle.checkStatus()

    expect(onError).toHaveBeenCalledWith(notFound)
    expect(lifecycle.getPendingRequestID()).toBe('test-request-id')
    expect(persistPendingRequestID).toHaveBeenCalledWith('test-request-id')
    expect(persistPendingRequestID).not.toHaveBeenCalledWith(null)
    expect(setTimeout).toHaveBeenCalled()
  })

  it('clears the persisted request ID when the mini program session resets', async () => {
    const { lifecycle, persistPendingRequestID } = createLifecycle()
    lifecycle.show()
    await lifecycle.submit({ model: 'gpt-mini', input: 'remove pending request after session reset' })

    lifecycle.resetSession()

    expect(lifecycle.getPendingRequestID()).toBeNull()
    expect(persistPendingRequestID).toHaveBeenLastCalledWith(null)
  })
})
