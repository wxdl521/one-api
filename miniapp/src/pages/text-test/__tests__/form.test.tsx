import { act, create, type ReactTestRenderer } from 'react-test-renderer'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const lifecycle = vi.hoisted(() => ({
  didHide: null as (() => void) | null,
  didShow: null as (() => void) | null,
  unload: null as (() => void) | null,
}))

const taro = vi.hoisted(() => ({
  reLaunch: vi.fn(),
  useDidHide: vi.fn((callback: () => void) => {
    lifecycle.didHide = callback
  }),
  useDidShow: vi.fn((callback: () => void) => {
    lifecycle.didShow = callback
  }),
  useUnload: vi.fn((callback: () => void) => {
    lifecycle.unload = callback
  }),
}))

const textTestService = vi.hoisted(() => ({
  createMiniTextTestRequestID: vi.fn().mockResolvedValue('test-request-id'),
  getMiniTextTestModels: vi.fn().mockResolvedValue(['gpt-mini']),
  getMiniTextTestStatus: vi.fn(),
  isMiniTextTestInputValid: vi.fn((input: string) => input.trim() !== '' && Array.from(input).length <= 4_000),
  miniTextTestMaxInputCodePoints: 4_000,
  startMiniTextTest: vi.fn(),
}))

vi.mock('@tarojs/components', () => ({
  Button: 'button',
  Input: 'input',
  Picker: 'picker',
  Text: 'text',
  Textarea: 'textarea',
  View: 'view',
}))
vi.mock('@tarojs/taro', () => ({ default: taro, ...taro }))
vi.mock('../../../features/text-test/text-test-service', () => textTestService)

import { t } from '../../../i18n/strings'
import TextTestPage from '../index'

async function flushPromises(): Promise<void> {
  await Promise.resolve()
  await Promise.resolve()
}

function findButton(renderer: ReactTestRenderer, label: string) {
  const button = renderer.root.findAllByType('button').find((candidate) => candidate.children.includes(label))
  if (button === undefined) {
    throw new Error(`Missing button ${label}`)
  }
  return button
}

describe('TextTestPage form', () => {
  beforeEach(() => {
    lifecycle.didHide = null
    lifecycle.didShow = null
    lifecycle.unload = null
    taro.reLaunch.mockReset()
    textTestService.createMiniTextTestRequestID.mockReset().mockResolvedValue('test-request-id')
    textTestService.getMiniTextTestModels.mockReset().mockResolvedValue(['gpt-mini'])
    textTestService.getMiniTextTestStatus.mockReset()
    textTestService.startMiniTextTest.mockReset()
  })

  it('renders a bounded text-only form with privacy guidance instead of an empty page', async () => {
    let renderer: ReactTestRenderer
    await act(async () => {
      renderer = create(<TextTestPage />)
      await flushPromises()
    })

    const textArea = renderer!.root.findByType('textarea')
    expect(textArea.props.maxlength).toBe(4_000)
    expect(JSON.stringify(renderer!.toJSON())).toContain(t('textTestPrivacy'))
    expect(JSON.stringify(renderer!.toJSON())).toContain(t('textTestNoAttachments'))
  })

  it('blocks an over-limit prompt locally without creating an idempotent request', async () => {
    let renderer: ReactTestRenderer
    await act(async () => {
      renderer = create(<TextTestPage />)
      await flushPromises()
    })
    const textArea = renderer!.root.findByType('textarea')
    await act(async () => {
      textArea.props.onInput({ detail: { value: '你'.repeat(4_001) } })
      findButton(renderer!, t('textTestSubmit')).props.onClick()
    })

    expect(textTestService.createMiniTextTestRequestID).not.toHaveBeenCalled()
    expect(JSON.stringify(renderer!.toJSON())).toContain(t('textTestInputLimit'))
  })
})
