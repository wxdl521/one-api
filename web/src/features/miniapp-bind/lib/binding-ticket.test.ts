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
  createMiniAppBindingConfirmationPayload,
  miniAppBindURLWithoutTicket,
} from './binding-ticket'

describe('mini program browser binding ticket handling', () => {
  test('sends only the opaque bind ticket and removes it from the browser URL', () => {
    assert.deepEqual(
      createMiniAppBindingConfirmationPayload('bind-flow-ticket'),
      { binding_ticket: 'bind-flow-ticket' }
    )
    assert.equal(
      miniAppBindURLWithoutTicket(
        '/miniapp-bind?binding_ticket=bind-flow-ticket#confirm'
      ),
      '/miniapp-bind#confirm'
    )
  })

  test('rejects missing, oversized, and substituted pending identity tickets', () => {
    assert.equal(createMiniAppBindingConfirmationPayload(''), null)
    assert.equal(
      createMiniAppBindingConfirmationPayload('x'.repeat(513)),
      null
    )
    assert.equal(
      miniAppBindURLWithoutTicket('/miniapp-bind?ticket=pending-ticket'),
      null
    )
  })
})
