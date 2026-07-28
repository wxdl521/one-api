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

import { filterChannelGroups, groupModelsByChannel } from '../channel-groups'

describe('groupModelsByChannel', () => {
  test('keeps a visible model in every channel that provides it', () => {
    const groups = groupModelsByChannel(
      [{ model_name: 'shared-model' }, { model_name: 'volcengine-model' }],
      [
        { name: 'Volcengine', models: ['shared-model', 'volcengine-model'] },
        { name: 'Mobile', models: ['shared-model'] },
      ]
    )

    assert.deepEqual(groups, [
      {
        name: 'Volcengine',
        models: [
          { model_name: 'shared-model' },
          { model_name: 'volcengine-model' },
        ],
      },
      { name: 'Mobile', models: [{ model_name: 'shared-model' }] },
    ])
  })

  test('removes channel sections with no models after filtering', () => {
    const groups = groupModelsByChannel(
      [{ model_name: 'volcengine-model' }],
      [
        { name: 'Volcengine', models: ['volcengine-model'] },
        { name: 'Mobile', models: ['mobile-model'] },
      ]
    )

    assert.deepEqual(groups, [
      { name: 'Volcengine', models: [{ model_name: 'volcengine-model' }] },
    ])
  })

  test('keeps only the selected channel section', () => {
    const groups = filterChannelGroups(
      [
        { name: 'Volcengine', models: [{ model_name: 'volcengine-model' }] },
        { name: 'Mobile', models: [{ model_name: 'mobile-model' }] },
      ],
      'Mobile'
    )

    assert.deepEqual(groups, [
      { name: 'Mobile', models: [{ model_name: 'mobile-model' }] },
    ])
  })
})
