import { act, create, type ReactTestRenderer } from 'react-test-renderer'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { MiniAppApiError } from '../../lib/api'
import { t } from '../../i18n/strings'

const taro = vi.hoisted(() => ({
  navigateTo: vi.fn(),
  reLaunch: vi.fn(),
}))

const auth = vi.hoisted(() => ({
  clearPendingIdentityTicket: vi.fn(),
  getPendingIdentityTicket: vi.fn(),
  registerWithPendingIdentity: vi.fn(),
}))

vi.mock('@tarojs/components', () => ({
  Button: 'button',
  Input: 'input',
  Text: 'text',
  View: 'view',
}))

vi.mock('@tarojs/taro', () => ({ default: taro, ...taro }))
vi.mock('../../features/auth/auth-service', () => auth)

import LoginPage from './index'

function getButton(renderer: ReactTestRenderer, label: string) {
  const button = renderer.root.findAllByType('button').find((candidate) => candidate.children.includes(label))
  if (button === undefined) {
    throw new Error(`Missing button ${label}`)
  }
  return button
}

function deferred<T>() {
  let reject: (reason?: unknown) => void = () => undefined
  const promise = new Promise<T>((_resolve, nextReject) => {
    reject = nextReject
  })
  return { promise, reject }
}

async function openRegistrationForm(renderer: ReactTestRenderer): Promise<void> {
  await act(async () => {
    getButton(renderer, t('register')).props.onClick()
  })
}

describe('LoginPage registration recovery UI', () => {
  beforeEach(() => {
    auth.clearPendingIdentityTicket.mockReset()
    auth.getPendingIdentityTicket.mockReset()
    auth.getPendingIdentityTicket.mockReturnValue('pending-ticket')
    auth.registerWithPendingIdentity.mockReset()
    taro.navigateTo.mockReset()
    taro.reLaunch.mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it.each([
    ['MINIAPP_REGISTRATION_DISABLED', 'registrationDisabled', 'continueBinding', 'bind-existing-account'],
    ['MINIAPP_TICKET_EXPIRED', 'registrationTicketInvalid', 'restartLogin', 'restart-login'],
    ['MINIAPP_TICKET_CONSUMED', 'registrationTicketInvalid', 'restartLogin', 'restart-login'],
    ['MINIAPP_EMAIL_VERIFICATION_REQUIRED', 'registrationVerificationRequired', 'restartVerification', 'restart-verification'],
  ] as const)('clears submission and renders the %s recovery action', async (code, messageKey, actionKey, action) => {
    const pendingRegistration = deferred<void>()
    auth.registerWithPendingIdentity.mockReturnValue(pendingRegistration.promise)
    let renderer: ReactTestRenderer
    await act(async () => {
      renderer = create(<LoginPage />)
    })

    await openRegistrationForm(renderer!)
    const submitButton = getButton(renderer!, t('register'))
    await act(async () => {
      submitButton.props.onClick()
      await Promise.resolve()
    })
    expect(getButton(renderer!, t('register')).props.loading).toBe(true)

    expect(renderer!.root.findAllByType('input')).toHaveLength(2)
    pendingRegistration.reject(new MiniAppApiError(code, 'safe error', 400))
    await act(async () => {
      await Promise.resolve()
    })

    expect(renderer!.root.findAllByType('input')).toHaveLength(0)
    expect(JSON.stringify(renderer!.toJSON())).toContain(t(messageKey))
    const recoveryButton = getButton(renderer!, t(actionKey))
    expect(recoveryButton.props.disabled).not.toBe(true)

    await act(async () => {
      recoveryButton.props.onClick()
      await Promise.resolve()
    })

    if (action === 'bind-existing-account') {
      expect(taro.navigateTo).toHaveBeenCalledWith({ url: '/pages/binding/index' })
      return
    }
    expect(auth.clearPendingIdentityTicket).toHaveBeenCalledOnce()
    expect(taro.reLaunch).toHaveBeenCalledWith({ url: '/pages/index/index' })
  })
})
