import { afterEach, describe, expect, it, vi } from 'vitest'

type MiniAppBuildConfig = typeof globalThis & {
  __MINIAPP_BINDING_ORIGIN__?: string
}

const buildConfig = globalThis as MiniAppBuildConfig

async function loadUserAgreementURL(origin?: string) {
  vi.resetModules()
  if (origin === undefined) {
    delete buildConfig.__MINIAPP_BINDING_ORIGIN__
  } else {
    buildConfig.__MINIAPP_BINDING_ORIGIN__ = origin
  }
  return import('./user-agreement')
}

afterEach(() => {
  delete buildConfig.__MINIAPP_BINDING_ORIGIN__
  vi.resetModules()
})

describe('Mini Program user agreement URL', () => {
  it('uses the exact public user agreement route on the configured HTTPS console origin', async () => {
    const { getMiniAppUserAgreementURL } = await loadUserAgreementURL('https://console.example.com')

    expect(getMiniAppUserAgreementURL()).toBe('https://console.example.com/user-agreement')
  })

  it.each([
    undefined,
    '',
    'http://console.example.com',
    'https://console.example.com/admin',
    'https://user:password@console.example.com',
    'https://console.example.com?redirect=https://attacker.example.com',
  ])('fails closed for an invalid public console origin: %s', async (origin) => {
    const { getMiniAppUserAgreementURL } = await loadUserAgreementURL(origin)

    expect(getMiniAppUserAgreementURL()).toBeNull()
  })
})
