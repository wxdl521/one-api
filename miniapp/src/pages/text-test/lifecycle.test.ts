import { describe, expect, it, vi } from 'vitest'

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

function createLifecycle(options?: { isRetryableError?: (error: unknown) => boolean }) {
  const createRequestID = vi.fn().mockResolvedValue('test-request-id')
  const getStatus = vi.fn().mockResolvedValue(runningStatus)
  const onError = vi.fn()
  const onPending = vi.fn()
  const onStatus = vi.fn()
  const start = vi.fn().mockResolvedValue(runningStatus)
  const clearTimeout = vi.fn()
  const setTimeout = vi.fn<(callback: () => void, delay: number) => ReturnType<typeof globalThis.setTimeout>>(
    () => 1 as unknown as ReturnType<typeof globalThis.setTimeout>,
  )
  const now = vi.fn(() => 0)
  const lifecycle = new TextTestLifecycle({
    createRequestID,
    getStatus,
    isRetryableError: options?.isRetryableError,
    onError,
    onPending,
    onStatus,
    start,
    clearTimeout,
    now,
    setTimeout,
  })
  return {
    lifecycle,
    createRequestID,
    getStatus,
    onError,
    onPending,
    onStatus,
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
    expect(lifecycle.getPendingRequestID()).toBeNull()
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
})
