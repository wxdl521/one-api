/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { PricingChannelGroup } from '../types'

export type ChannelModelGroup<T extends { model_name: string }> = {
  name: string
  models: T[]
}

export function groupModelsByChannel<T extends { model_name: string }>(
  models: T[],
  channelGroups: PricingChannelGroup[]
): ChannelModelGroup<T>[] {
  const modelsByName = new Map(models.map((model) => [model.model_name, model]))

  return channelGroups.flatMap((channelGroup) => {
    const channelModels = channelGroup.models.flatMap((modelName) => {
      const model = modelsByName.get(modelName)
      return model ? [model] : []
    })

    if (channelModels.length === 0) {
      return []
    }

    return [{ name: channelGroup.name, models: channelModels }]
  })
}

export function filterChannelGroups<T extends { model_name: string }>(
  groups: ChannelModelGroup<T>[],
  channelName: string
): ChannelModelGroup<T>[] {
  if (!channelName) {
    return groups
  }
  return groups.filter((group) => group.name === channelName)
}
