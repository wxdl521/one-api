import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from './channel-form'

test('sends VolcEngine Agent Plan credentials without mixing them into the relay API key', () => {
  const values = {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    type: 58,
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
    { ...CHANNEL_FORM_DEFAULT_VALUES, type: 58 },
    1
  )

  assert.equal(payload.agent_plan_access_key, undefined)
  assert.equal(payload.agent_plan_secret_key, undefined)
})
