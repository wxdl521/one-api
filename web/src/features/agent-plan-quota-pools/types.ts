export interface AgentPlanQuotaPoolStock {
  official_monthly_remaining_afp_micros: number
  active_reservation_afp_micros: number
  sellable_afp_micros: number
}

export interface AgentPlanQuotaPool {
  id: number
  name: string
  source_channel_id: number
  display_multiplier: number
  official_monthly_remaining_afp_micros: number
  five_hour_remaining_afp_micros: number
  five_hour_reset_at: number
  weekly_remaining_afp_micros: number
  weekly_reset_at: number
  monthly_reset_at: number
  synced_at: number
  sync_status: 'available' | 'error'
  stock: AgentPlanQuotaPoolStock
}

export interface AgentPlanEligibleSourceChannel {
  id: number
  name: string
  type: number
}

export interface AgentPlanQuotaPoolPayload {
  name: string
  source_channel_id: number
  display_multiplier: number
}
