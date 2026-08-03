import { act, create, type ReactTestRenderer } from 'react-test-renderer'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { MiniAppApiError } from '../../../lib/api'
import { t } from '../../../i18n/strings'

const taro = vi.hoisted(() => ({
  reLaunch: vi.fn(),
}))

const accountService = vi.hoisted(() => ({
  getAccountOverview: vi.fn(),
}))

vi.mock('@tarojs/components', () => ({
  Button: 'button',
  Text: 'text',
  View: 'view',
}))

vi.mock('@tarojs/taro', () => ({ default: taro, ...taro }))
vi.mock('../../../features/account/account-service', () => accountService)

import AccountPage from '../index'

function getButton(renderer: ReactTestRenderer, label: string) {
  const button = renderer.root.findAllByType('button').find((candidate) => candidate.children.includes(label))
  if (button === undefined) {
    throw new Error(`Missing button ${label}`)
  }
  return button
}

function deferred<T>() {
  let resolve: (value: T) => void = () => undefined
  let reject: (reason?: unknown) => void = () => undefined
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve
    reject = nextReject
  })
  return { promise, resolve, reject }
}

async function flushPromises(): Promise<void> {
  await Promise.resolve()
  await Promise.resolve()
}

const overview = {
  username: 'mini-user',
  displayName: 'Mini User',
  email: 'm***@example.com',
  quota: { balance: 1200, unit: 'quota' as const },
  enabledGroups: ['default'],
  subscriptions: [{
    planTitle: 'Monthly plan',
    status: 'active',
    endsAt: 1_800_000_000,
    quota: { remaining: 300, unlimited: false, unit: 'quota' as const },
  }],
}

describe('AccountPage overview', () => {
  beforeEach(() => {
    accountService.getAccountOverview.mockReset()
    taro.reLaunch.mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows a loading state before rendering the display-only overview', async () => {
    const pendingOverview = deferred<typeof overview>()
    accountService.getAccountOverview.mockReturnValue(pendingOverview.promise)
    let renderer: ReactTestRenderer
    await act(async () => {
      renderer = create(<AccountPage />)
      await flushPromises()
    })

    expect(JSON.stringify(renderer!.toJSON())).toContain(t('accountLoading'))
    pendingOverview.resolve(overview)
    await act(async () => {
      await flushPromises()
    })

    const output = JSON.stringify(renderer!.toJSON())
    expect(output).toContain('Mini User')
    expect(output).toContain('m***@example.com')
    expect(output).toContain('Monthly plan')
    expect(renderer!.root.findAllByType('input')).toHaveLength(0)
  })

  it('shows the subscription empty state and lets the user refresh it manually', async () => {
    accountService.getAccountOverview.mockResolvedValue({ ...overview, subscriptions: [] })
    let renderer: ReactTestRenderer
    await act(async () => {
      renderer = create(<AccountPage />)
      await flushPromises()
    })

    expect(JSON.stringify(renderer!.toJSON())).toContain(t('accountNoSubscriptions'))
    expect(accountService.getAccountOverview).toHaveBeenCalledTimes(1)
    await act(async () => {
      getButton(renderer!, t('refresh')).props.onClick()
      await flushPromises()
    })
    expect(accountService.getAccountOverview).toHaveBeenCalledTimes(2)
  })

  it('shows a retryable error when the overview request fails', async () => {
    accountService.getAccountOverview.mockRejectedValue(new Error('network unavailable'))
    let renderer: ReactTestRenderer
    await act(async () => {
      renderer = create(<AccountPage />)
      await flushPromises()
    })

    expect(JSON.stringify(renderer!.toJSON())).toContain(t('accountLoadFailed'))
    expect(getButton(renderer!, t('refresh')).props.disabled).not.toBe(true)
  })

  it.each([
    ['MINIAPP_SESSION_INVALID', 401],
    ['MINIAPP_SESSION_UNAVAILABLE', 0],
  ])('returns to login when the Mini Program session is unavailable: %s', async (code, status) => {
    accountService.getAccountOverview.mockRejectedValue(new MiniAppApiError(code, 'Unauthorized', status))
    await act(async () => {
      create(<AccountPage />)
      await flushPromises()
    })

    expect(taro.reLaunch).toHaveBeenCalledWith({ url: '/pages/index/index' })
  })
})
