/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  clearMiniAppBindingSessionTicket,
  consumeMiniAppBindingBootstrapTicket,
  consumeMiniAppBindingURL,
  createMiniAppBindingConfirmationPayload,
  miniAppBindingTicketBootstrapWindowKey,
  readMiniAppBindingSessionTicket,
  rememberMiniAppBindingSessionTicket,
} from './binding-ticket'

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

describe('mini program browser binding ticket handling', () => {
  test('captures the bind ticket from the fragment and removes it from the visible URL', () => {
    assert.deepEqual(
      createMiniAppBindingConfirmationPayload('bind-flow-ticket'),
      { binding_ticket: 'bind-flow-ticket' }
    )
    assert.deepEqual(
      consumeMiniAppBindingURL('/miniapp-bind#binding_ticket=bind-flow-ticket'),
      {
        bindingTicket: 'bind-flow-ticket',
        visibleURL: '/miniapp-bind',
      }
    )
  })

  test('rejects query tickets and always removes opaque data from the visible URL', () => {
    assert.equal(createMiniAppBindingConfirmationPayload(''), null)
    assert.equal(createMiniAppBindingConfirmationPayload('x'.repeat(513)), null)
    assert.deepEqual(
      consumeMiniAppBindingURL('/miniapp-bind?binding_ticket=bind-flow-ticket'),
      { bindingTicket: null, visibleURL: '/miniapp-bind' }
    )
    assert.deepEqual(
      consumeMiniAppBindingURL('/miniapp-bind#ticket=pending-ticket'),
      { bindingTicket: null, visibleURL: '/miniapp-bind' }
    )
  })

  test('consumes the in-memory bootstrap ticket once without persistence', () => {
    const handoff = {
      [miniAppBindingTicketBootstrapWindowKey]: 'bind-flow-ticket',
    }

    assert.equal(
      consumeMiniAppBindingBootstrapTicket(handoff),
      'bind-flow-ticket'
    )
    assert.equal(
      Object.hasOwn(handoff, miniAppBindingTicketBootstrapWindowKey),
      false
    )
    assert.equal(consumeMiniAppBindingBootstrapTicket(handoff), null)
  })

  test('keeps a valid ticket only in this browser session through sign-in continuation', () => {
    const storage = createSessionStorage()

    assert.equal(
      rememberMiniAppBindingSessionTicket('bind-flow-ticket', storage),
      true
    )
    assert.equal(readMiniAppBindingSessionTicket(storage), 'bind-flow-ticket')
    clearMiniAppBindingSessionTicket(storage)
    assert.equal(readMiniAppBindingSessionTicket(storage), null)
  })

  test('does not persist malformed tickets', () => {
    const storage = createSessionStorage()

    assert.equal(rememberMiniAppBindingSessionTicket('   ', storage), false)
    assert.equal(readMiniAppBindingSessionTicket(storage), null)
  })
})
