import type { AgentPlanUsageResponse } from '../api'
import type { Channel } from '../types'
import { parseChannelOtherSettings } from './channel-utils'

const AGENT_PLAN_CHANNEL_TYPES = new Set([45, 58, 60])

export function isAgentPlanUsageEnabled(channel: Channel): boolean {
  const settings = parseChannelOtherSettings(channel.settings)
  return (
    AGENT_PLAN_CHANNEL_TYPES.has(channel.type) &&
    !channel.channel_info.is_multi_key &&
    (channel.type === 60
      ? settings.agent_plan_usage_enabled !== false
      : settings.agent_plan_usage_enabled === true)
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
