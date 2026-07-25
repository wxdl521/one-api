import assert from 'node:assert/strict'
import { test } from 'node:test'

import type { AgentPlanUsageResponse } from '../api'
import type { Channel } from '../types'
import {
  isAgentPlanUsageEnabled,
  loadAgentPlanUsageSafely,
} from './agent-plan-usage'

function channel(overrides: Partial<Channel>): Channel {
  const base: Channel = {
    id: 1,
    type: 58,
    key: '',
    status: 1,
    name: 'Volcano',
    created_time: 0,
    test_time: 0,
    response_time: 0,
    other: '',
    balance: 0,
    balance_updated_time: 0,
    models: '',
    group: 'default',
    used_quota: 0,
    other_info: '',
    remark: '',
    max_input_tokens: 0,
    settings: '{}',
    channel_info: {
      is_multi_key: false,
      multi_key_size: 0,
      multi_key_polling_index: 0,
      multi_key_mode: 'random',
    },
  }
  return Object.assign(base, overrides)
}

test('loads AFP usage only for enabled single-key VolcEngine channels', () => {
  assert.equal(
    isAgentPlanUsageEnabled(
      channel({ settings: '{"agent_plan_usage_enabled":true}' })
    ),
    true
  )
  assert.equal(
    isAgentPlanUsageEnabled(
      channel({ type: 1, settings: '{"agent_plan_usage_enabled":true}' })
    ),
    false
  )
  assert.equal(
    isAgentPlanUsageEnabled(
      channel({
        settings: '{"agent_plan_usage_enabled":true}',
        channel_info: {
          is_multi_key: true,
          multi_key_size: 2,
          multi_key_polling_index: 0,
          multi_key_mode: 'random',
        },
      })
    ),
    false
  )
})

test('contains Agent Plan usage request failures within the card view', async () => {
  const response = await loadAgentPlanUsageSafely(async () => {
    throw new Error('upstream unavailable')
  })

  assert.deepEqual(response, { success: false, data: {} } satisfies AgentPlanUsageResponse)
})
