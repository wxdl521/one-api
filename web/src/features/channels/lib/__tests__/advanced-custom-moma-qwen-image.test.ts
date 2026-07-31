import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  ADVANCED_CUSTOM_CONVERTER_OPTIONS,
  getAdvancedCustomConverterDefaults,
  getDefaultAdvancedCustomIncomingPath,
  validateAdvancedCustomConfig,
} from '../advanced-custom'

const converter = 'openai_image_to_moma_qwen_image' as const

test('registers the MoMA Qwen Image converter with its documented defaults', () => {
  assert.ok(
    ADVANCED_CUSTOM_CONVERTER_OPTIONS.some(
      (option) => option.value === converter
    )
  )
  assert.equal(
    getDefaultAdvancedCustomIncomingPath(converter),
    '/v1/images/generations'
  )
  assert.deepEqual(
    getAdvancedCustomConverterDefaults(converter, '/v1/images/generations'),
    {
      upstream_path: '/v1/aigc/multimodal-generation/generation',
      auth: {
        type: 'header',
        name: 'Authorization',
        value: 'Bearer {api_key}',
      },
    }
  )
})

test('rejects the MoMA Qwen Image converter on a non-image route', () => {
  assert.deepEqual(
    validateAdvancedCustomConfig({
      advanced_routes: [
        {
          incoming_path: '/v1/chat/completions',
          upstream_path: '/v1/aigc/multimodal-generation/generation',
          converter,
          models: ['qwen/qwen-image-2.0-pro'],
        },
      ],
    }),
    {
      routeIndex: 0,
      message: 'Converter does not match incoming path',
    }
  )
})
