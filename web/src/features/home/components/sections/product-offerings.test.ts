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
import test from 'node:test'

import { productOfferings } from './product-offerings'

test('exposes the three promoted products with their published packages', () => {
  assert.deepEqual(
    productOfferings.map((offering) => offering.name),
    ['ArkClaw', 'Trae Work', 'Agent Plan']
  )
  assert.equal(productOfferings[0].priceKey, '¥210')
  assert.equal(productOfferings[1].priceKey, '¥149 / seat / month')
  assert.equal(productOfferings[2].priceKey, '¥200 / month')
  assert.ok(
    productOfferings[2].details.includes('100,000 AFP monthly allowance')
  )
})
