import { describe, expect, it } from 'vitest'

import appConfig from './app.config'

describe('phase-one route manifest', () => {
  it('keeps the authentication bootstrap as the application entry page', () => {
    expect(appConfig.entryPagePath).toBe('pages/index/index')
  })

  it('declares only the planned mini program routes', () => {
    expect(appConfig.pages).toEqual([
      'pages/index/index',
      'pages/login/index',
      'pages/binding/index',
      'pages/account/index',
      'pages/tokens/index',
      'pages/products/index',
      'pages/orders/index',
      'pages/text-test/index',
    ])
  })
})
