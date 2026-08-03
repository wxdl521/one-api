import { afterEach, describe, expect, it } from 'vitest'

import {
  clearMiniAppSession,
  getMiniAppSession,
  setMiniAppSession,
} from './session'

describe('mini program session', () => {
  afterEach(() => {
    clearMiniAppSession()
  })

  it('keeps an issued session only in process memory', () => {
    setMiniAppSession({
      accessToken: 'short-lived-access-token',
      accessExpiresAt: Math.floor(Date.now() / 1000) + 60,
      sid: 'miniapp-session-id',
    })

    expect(getMiniAppSession()).toEqual({
      accessToken: 'short-lived-access-token',
      accessExpiresAt: expect.any(Number),
      sid: 'miniapp-session-id',
    })

    clearMiniAppSession()
    expect(getMiniAppSession()).toBeNull()
  })

  it('drops an expired session instead of returning a stale bearer token', () => {
    setMiniAppSession({
      accessToken: 'expired-access-token',
      accessExpiresAt: Math.floor(Date.now() / 1000) - 1,
      sid: 'expired-session-id',
    })

    expect(getMiniAppSession()).toBeNull()
    expect(getMiniAppSession()).toBeNull()
  })
})
