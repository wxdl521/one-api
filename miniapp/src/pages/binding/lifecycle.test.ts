import { describe, expect, it, vi } from 'vitest'

import { BindingLifecycle } from './lifecycle'

function deferred<T>() {
  let resolve: (value: T) => void = () => undefined
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve
  })
  return { promise, resolve }
}

async function flushPromises() {
  await Promise.resolve()
  await Promise.resolve()
}

function createLifecycle(options?: {
  createBinding?: () => Promise<{ bindingId: string; webUrl: string }>
  getStatus?: () => Promise<'pending' | 'bound' | 'expired'>
}) {
  const onBound = vi.fn()
  const onError = vi.fn()
  const onLoading = vi.fn()
  const onReady = vi.fn()
  const clearInterval = vi.fn()
  const clearTimeout = vi.fn()
  const setInterval = vi.fn(() => 1 as unknown as ReturnType<typeof globalThis.setInterval>)
  const setTimeout = vi.fn(() => 2 as unknown as ReturnType<typeof globalThis.setTimeout>)
  const lifecycle = new BindingLifecycle({
    createBinding: options?.createBinding ?? (async () => ({ bindingId: 'binding-id', webUrl: 'https://console.example.com/miniapp-bind#ticket' })),
    getStatus: options?.getStatus ?? (async () => 'pending'),
    onBound,
    onError,
    onLoading,
    onReady,
    setInterval,
    clearInterval,
    setTimeout,
    clearTimeout,
  })
  return { lifecycle, onBound, onError, onReady, setInterval, clearInterval }
}

describe('binding page lifecycle', () => {
  it('does not schedule or mutate the page when a delayed create resolves after hide', async () => {
    const pendingCreate = deferred<{ bindingId: string; webUrl: string }>()
    const { lifecycle, onReady, setInterval } = createLifecycle({ createBinding: () => pendingCreate.promise })

    const showing = lifecycle.show()
    lifecycle.hide()
    pendingCreate.resolve({ bindingId: 'binding-id', webUrl: 'https://console.example.com/miniapp-bind#ticket' })
    await showing

    expect(onReady).not.toHaveBeenCalled()
    expect(setInterval).not.toHaveBeenCalled()
  })

  it('ignores a delayed poll result after unload', async () => {
    const pendingStatus = deferred<'pending' | 'bound' | 'expired'>()
    const { lifecycle, onBound, onError, setInterval, clearInterval } = createLifecycle({
      getStatus: () => pendingStatus.promise,
    })

    await lifecycle.show()
    await flushPromises()
    lifecycle.unload()
    pendingStatus.resolve('bound')
    await flushPromises()

    expect(setInterval).toHaveBeenCalledTimes(1)
    expect(clearInterval).toHaveBeenCalledTimes(1)
    expect(onBound).not.toHaveBeenCalled()
    expect(onError).not.toHaveBeenCalled()
  })
})
