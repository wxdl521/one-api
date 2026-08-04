import Taro from '@tarojs/taro'

import { MiniAppApiError, request } from '../../lib/api'

export const miniTextTestMaxInputCodePoints = 4_000

export type MiniTextTestState = 'running' | 'succeeded' | 'failed' | 'timed_out'

export interface MiniTextTestStatus {
  requestID: string
  model: string
  state: MiniTextTestState
  chargeReference: string
  chargedQuota: number
  errorCode: string
  createdAt: number
  startedAt: number
  completedAt: number
}

export interface StartMiniTextTestInput {
  clientRequestID: string
  model: string
  input: string
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object'
}

function invalidTextTestResponse(): MiniAppApiError {
  return new MiniAppApiError('MINIAPP_INVALID_TEXT_TEST_RESPONSE', 'Mini Program text-test response is invalid')
}

function readTextTestState(value: unknown): MiniTextTestState | null {
  if (value === 'running' || value === 'succeeded' || value === 'failed' || value === 'timed_out') {
    return value
  }
  return null
}

function readTextTestStatus(value: unknown): MiniTextTestStatus {
  if (!isRecord(value)) {
    throw invalidTextTestResponse()
  }
  const state = readTextTestState(value.state)
  if (
    typeof value.request_id !== 'string' || value.request_id.trim() === '' ||
    typeof value.model !== 'string' || value.model.trim() === '' ||
    state === null ||
    (value.charge_ref !== undefined && typeof value.charge_ref !== 'string') ||
    typeof value.charged_quota !== 'number' || !Number.isSafeInteger(value.charged_quota) || value.charged_quota < 0 ||
    (value.error_code !== undefined && typeof value.error_code !== 'string') ||
    typeof value.created_at !== 'number' || !Number.isSafeInteger(value.created_at) || value.created_at < 0 ||
    typeof value.started_at !== 'number' || !Number.isSafeInteger(value.started_at) || value.started_at < 0 ||
    (value.completed_at !== undefined && (
      typeof value.completed_at !== 'number' || !Number.isSafeInteger(value.completed_at) || value.completed_at < 0
    ))
  ) {
    throw invalidTextTestResponse()
  }
  return {
    requestID: value.request_id,
    model: value.model,
    state,
    chargeReference: value.charge_ref ?? '',
    chargedQuota: value.charged_quota,
    errorCode: value.error_code ?? '',
    createdAt: value.created_at,
    startedAt: value.started_at,
    completedAt: value.completed_at ?? 0,
  }
}

export function isMiniTextTestInputValid(input: string): boolean {
  return input.trim() !== '' && Array.from(input).length <= miniTextTestMaxInputCodePoints
}

export async function createMiniTextTestRequestID(): Promise<string> {
  const result = await Taro.getRandomValues({ length: 16 })
  const bytes = new Uint8Array(result.randomValues)
  if (bytes.length !== 16) {
    throw new MiniAppApiError('MINIAPP_RANDOM_UNAVAILABLE', 'Mini Program secure random values are unavailable')
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

export async function getMiniTextTestModels(): Promise<string[]> {
  const response = await request<unknown>({ path: '/text-test/models', method: 'GET', auth: 'session' })
  if (!isRecord(response) || !Array.isArray(response.models) || !response.models.every((model) => typeof model === 'string' && model.trim() !== '')) {
    throw invalidTextTestResponse()
  }
  return response.models.slice()
}

export async function startMiniTextTest(input: StartMiniTextTestInput): Promise<MiniTextTestStatus> {
  const response = await request<unknown, {
    client_request_id: string
    model: string
    input: string
  }>({
    path: '/text-tests',
    method: 'POST',
    data: {
      client_request_id: input.clientRequestID,
      model: input.model,
      input: input.input,
    },
    auth: 'session',
    timeout: 20_000,
  })
  return readTextTestStatus(response)
}

export async function getMiniTextTestStatus(requestID: string): Promise<MiniTextTestStatus> {
  return readTextTestStatus(await request<unknown>({
    path: `/text-tests/${encodeURIComponent(requestID)}`,
    method: 'GET',
    auth: 'session',
  }))
}
