import { act, create } from 'react-test-renderer'
import { describe, expect, it, vi } from 'vitest'

const tokenService = vi.hoisted(() => ({
  createMiniAppToken: vi.fn(),
  getMiniAppTokens: vi.fn().mockResolvedValue([]),
  revokeMiniAppToken: vi.fn(),
  updateMiniAppTokenStatus: vi.fn(),
}))

const lifecycle = vi.hoisted(() => ({
  didHide: null as (() => void) | null,
}))

const taro = vi.hoisted(() => ({
  reLaunch: vi.fn(),
  setClipboardData: vi.fn(),
  showModal: vi.fn(),
  useDidHide: vi.fn((callback: () => void) => {
    lifecycle.didHide = callback
  }),
}))

vi.mock('@tarojs/components', () => ({
  Button: 'button',
  Input: 'input',
  Picker: 'picker',
  Text: 'text',
  View: 'view',
}))

vi.mock('@tarojs/taro', () => ({
  default: taro,
  ...taro,
}))

vi.mock('../../../features/tokens/token-service', () => tokenService)

import { t } from '../../../i18n/strings'
import TokensPage from '../index'

describe('TokensPage lifecycle', () => {
  it('renders the Mini Program token management surface instead of an empty page', () => {
    const renderer = create(<TokensPage />)

    expect(JSON.stringify(renderer.toJSON())).toContain(t('tokens'))
  })

  it('keeps a newly created raw key only until the user marks it saved', async () => {
    tokenService.createMiniAppToken.mockResolvedValue({
      token: {
        id: 8,
        name: 'daily',
        keyHint: 'sk-...1234',
        status: 1,
        createdAt: 1,
        accessedAt: 0,
        expiresAt: 2,
        group: 'default',
        modelLimits: ['gpt-4.1-mini'],
      },
      tokenKey: 'raw-miniapp-token-key',
    })
    const renderer = create(<TokensPage />)

    await act(async () => {
      await Promise.resolve()
    })
    const inputs = renderer.root.findAllByType('input')
    await act(async () => {
      inputs[0].props.onInput({ detail: { value: 'daily' } })
      inputs[1].props.onInput({ detail: { value: 'default' } })
      inputs[2].props.onInput({ detail: { value: 'gpt-4.1-mini' } })
    })
    const createButton = renderer.root.findAllByType('button').find((button) => button.children.includes(t('tokenCreate')))
    expect(createButton).toBeDefined()
    await act(async () => {
      createButton?.props.onClick()
      await Promise.resolve()
    })
    expect(JSON.stringify(renderer.toJSON())).toContain('raw-miniapp-token-key')

    const savedButton = renderer.root.findAllByType('button').find((button) => button.children.includes(t('tokenCreatedKeySaved')))
    expect(savedButton).toBeDefined()
    await act(async () => {
      savedButton?.props.onClick()
    })
    expect(JSON.stringify(renderer.toJSON())).not.toContain('raw-miniapp-token-key')
  })

  it('clears a raw key when the page is hidden', async () => {
    tokenService.createMiniAppToken.mockResolvedValue({
      token: {
        id: 9,
        name: 'mobile',
        keyHint: 'sk-...4321',
        status: 1,
        createdAt: 1,
        accessedAt: 0,
        expiresAt: 2,
        group: 'default',
        modelLimits: ['gpt-4.1-mini'],
      },
      tokenKey: 'raw-key-to-hide',
    })
    const renderer = create(<TokensPage />)

    await act(async () => {
      await Promise.resolve()
    })
    const inputs = renderer.root.findAllByType('input')
    await act(async () => {
      inputs[0].props.onInput({ detail: { value: 'mobile' } })
      inputs[1].props.onInput({ detail: { value: 'default' } })
      inputs[2].props.onInput({ detail: { value: 'gpt-4.1-mini' } })
    })
    const createButton = renderer.root.findAllByType('button').find((button) => button.children.includes(t('tokenCreate')))
    await act(async () => {
      createButton?.props.onClick()
      await Promise.resolve()
    })
    expect(JSON.stringify(renderer.toJSON())).toContain('raw-key-to-hide')

    await act(async () => {
      lifecycle.didHide?.()
    })
    expect(JSON.stringify(renderer.toJSON())).not.toContain('raw-key-to-hide')
  })
})
