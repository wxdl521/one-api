import assert from 'node:assert/strict'
import { test } from 'node:test'

import { getQuotaPoolStockValues } from '../lib'
import type { AgentPlanQuotaPool } from '../types'

test('shows sellable AFP after existing package reservations', () => {
  const pool: AgentPlanQuotaPool = {
    id: 1,
    name: 'Volc pool',
    source_channel_id: 3,
    display_multiplier: 2,
    official_monthly_remaining_afp_micros: 1_000_000_000,
    five_hour_remaining_afp_micros: 0,
    five_hour_reset_at: 0,
    weekly_remaining_afp_micros: 0,
    weekly_reset_at: 0,
    monthly_reset_at: 0,
    synced_at: 0,
    sync_status: 'available',
    stock: {
      official_monthly_remaining_afp_micros: 1_000_000_000,
      active_reservation_afp_micros: 400_000_000,
      sellable_afp_micros: 600_000_000,
    },
  }

  assert.deepEqual(getQuotaPoolStockValues(pool), {
    reservedAFPMicros: 400_000_000,
    sellableAFPMicros: 600_000_000,
  })
})
