import type { AgentPlanUsageResponse } from '../api'
import type { Channel } from '../types'
import { parseChannelOtherSettings } from './channel-utils'

const AGENT_PLAN_CHANNEL_TYPES = new Set([45, 58])

export function isAgentPlanUsageEnabled(channel: Channel): boolean {
  return (
    AGENT_PLAN_CHANNEL_TYPES.has(channel.type) &&
    !channel.channel_info.is_multi_key &&
    parseChannelOtherSettings(channel.settings).agent_plan_usage_enabled === true
  )
}

export async function loadAgentPlanUsageSafely(
  fetchUsage: () => Promise<AgentPlanUsageResponse>
): Promise<AgentPlanUsageResponse> {
  try {
    return await fetchUsage()
  } catch {
    return { success: false, data: {} }
  }
}
