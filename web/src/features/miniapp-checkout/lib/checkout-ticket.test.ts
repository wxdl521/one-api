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
  consumeMiniAppCheckoutBootstrapTicket,
  consumeMiniAppCheckoutURL,
  createMiniAppCheckoutConfirmationPayload,
  miniAppCheckoutTicketBootstrapWindowKey,
} from './checkout-ticket'

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
})
