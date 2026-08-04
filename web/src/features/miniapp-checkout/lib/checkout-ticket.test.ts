/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  clearRememberedMiniAppCheckoutTicket,
  consumeMiniAppCheckoutBootstrapTicket,
  consumeMiniAppCheckoutURL,
  createMiniAppCheckoutConfirmationPayload,
  getRememberedMiniAppCheckoutTicket,
  miniAppCheckoutTicketBootstrapWindowKey,
  rememberMiniAppCheckoutTicket,
} from './checkout-ticket'

function createSessionStorage() {
  const values = new Map<string, string>()
  return {
    getItem(key: string) {
      return values.get(key) ?? null
    },
    setItem(key: string, value: string) {
      values.set(key, value)
    },
    removeItem(key: string) {
      values.delete(key)
    },
  }
}

describe('mini program browser checkout ticket handling', () => {
  test('captures the checkout ticket from a fragment without retaining it in the visible URL', () => {
    assert.deepEqual(
      createMiniAppCheckoutConfirmationPayload('checkout-flow-ticket'),
      {
        checkout_ticket: 'checkout-flow-ticket',
      }
    )
    assert.deepEqual(
      consumeMiniAppCheckoutURL(
        '/miniapp-checkout#checkout_ticket=checkout-flow-ticket'
      ),
      {
        checkoutTicket: 'checkout-flow-ticket',
        visibleURL: '/miniapp-checkout',
      }
    )
  })

  test('rejects query tickets and unexpected fragment values', () => {
    assert.equal(createMiniAppCheckoutConfirmationPayload(''), null)
    assert.equal(
      createMiniAppCheckoutConfirmationPayload('x'.repeat(513)),
      null
    )
    assert.deepEqual(
      consumeMiniAppCheckoutURL('/miniapp-checkout?checkout_ticket=ticket'),
      {
        checkoutTicket: null,
        visibleURL: '/miniapp-checkout',
      }
    )
    assert.deepEqual(
      consumeMiniAppCheckoutURL('/miniapp-checkout#binding_ticket=ticket'),
      {
        checkoutTicket: null,
        visibleURL: '/miniapp-checkout',
      }
    )
  })

  test('consumes the in-memory bootstrap ticket once without persistence', () => {
    const handoff = {
      [miniAppCheckoutTicketBootstrapWindowKey]: 'checkout-flow-ticket',
    }

    assert.equal(
      consumeMiniAppCheckoutBootstrapTicket(handoff),
      'checkout-flow-ticket'
    )
    assert.equal(
      Object.hasOwn(handoff, miniAppCheckoutTicketBootstrapWindowKey),
      false
    )
    assert.equal(consumeMiniAppCheckoutBootstrapTicket(handoff), null)
  })

  test('retains an opaque ticket through a browser auth navigation without a URL', () => {
    const storage = createSessionStorage()

    assert.equal(
      rememberMiniAppCheckoutTicket('checkout-flow-ticket', storage),
      'checkout-flow-ticket'
    )
    assert.equal(
      getRememberedMiniAppCheckoutTicket(storage),
      'checkout-flow-ticket'
    )
    clearRememberedMiniAppCheckoutTicket(storage)
    assert.equal(getRememberedMiniAppCheckoutTicket(storage), null)
  })
})
