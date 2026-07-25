import type { AgentPlanQuotaPool } from './types'

export function getQuotaPoolStockValues(pool: AgentPlanQuotaPool): {
  reservedAFPMicros: number
  sellableAFPMicros: number
} {
  return {
    reservedAFPMicros: pool.stock.active_reservation_afp_micros,
    sellableAFPMicros: pool.stock.sellable_afp_micros,
  }
}
