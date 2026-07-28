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
import { useTranslation } from 'react-i18next'

import { DEFAULT_TOKEN_UNIT, VIEW_MODES, type ViewMode } from '../constants'
import type { ChannelModelGroup } from '../lib/channel-groups'
import type { PricingModel, TokenUnit } from '../types'
import { ModelCardGrid } from './model-card-grid'
import { PricingTable } from './pricing-table'

export interface ChannelPricingSectionsProps {
  groups: ChannelModelGroup<PricingModel>[]
  viewMode: ViewMode
  onModelClick: (modelName: string) => void
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
}

export function ChannelPricingSections(props: ChannelPricingSectionsProps) {
  const { t } = useTranslation()
  const tokenUnit = props.tokenUnit ?? DEFAULT_TOKEN_UNIT

  return (
    <div className='space-y-8'>
      {props.groups.map((group) => (
        <section key={group.name} aria-label={group.name} className='space-y-3'>
          <header className='flex items-baseline justify-between gap-3 border-b pb-2'>
            <h2 className='text-lg font-semibold'>{group.name}</h2>
            <span className='text-muted-foreground text-sm tabular-nums'>
              {t('{{count}} models', { count: group.models.length })}
            </span>
          </header>
          {props.viewMode === VIEW_MODES.CARD ? (
            <ModelCardGrid
              models={group.models}
              onModelClick={props.onModelClick}
              priceRate={props.priceRate}
              usdExchangeRate={props.usdExchangeRate}
              tokenUnit={tokenUnit}
              showRechargePrice={props.showRechargePrice}
            />
          ) : (
            <PricingTable
              models={group.models}
              onModelClick={props.onModelClick}
              priceRate={props.priceRate}
              usdExchangeRate={props.usdExchangeRate}
              tokenUnit={tokenUnit}
              showRechargePrice={props.showRechargePrice}
            />
          )}
        </section>
      ))}
    </div>
  )
}
