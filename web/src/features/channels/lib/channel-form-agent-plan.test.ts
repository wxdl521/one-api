import assert from 'node:assert/strict'
import { test } from 'node:test'

import type { Channel } from '../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from './channel-form'

test('sends VolcEngine Agent Plan credentials without mixing them into the relay API key', () => {
  const values = {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    type: 60,
    key: 'relay-api-key',
    agent_plan_access_key: 'AKEXAMPLE',
    agent_plan_secret_key: 'secret-example',
  }

  const createPayload = transformFormDataToCreatePayload(values)
  assert.equal(createPayload.channel.key, 'relay-api-key')
  assert.equal(createPayload.channel.agent_plan_access_key, 'AKEXAMPLE')
  assert.equal(createPayload.channel.agent_plan_secret_key, 'secret-example')

  const updatePayload = transformFormDataToUpdatePayload(
    { ...values, key: '' },
    1
  )
  assert.equal(updatePayload.key, undefined)
  assert.equal(updatePayload.agent_plan_access_key, 'AKEXAMPLE')
  assert.equal(updatePayload.agent_plan_secret_key, 'secret-example')
})

test('does not submit blank Agent Plan credential fields when editing a channel', () => {
  const payload = transformFormDataToUpdatePayload(
    { ...CHANNEL_FORM_DEFAULT_VALUES, type: 60 },
    1
  )

  assert.equal(payload.agent_plan_access_key, undefined)
  assert.equal(payload.agent_plan_secret_key, undefined)
})

test('defaults usage on for an Agent Plan channel without saved settings', () => {
  const channel = {
    id: 1,
    type: 60,
    key: '',
    name: 'Agent Plan',
    status: 1,
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
    settings: '',
    channel_info: {
      is_multi_key: false,
      multi_key_size: 0,
      multi_key_polling_index: 0,
      multi_key_mode: 'random',
    },
  } satisfies Channel

  assert.equal(
    transformChannelToFormDefaults(channel).agent_plan_usage_enabled,
    true
  )
})
