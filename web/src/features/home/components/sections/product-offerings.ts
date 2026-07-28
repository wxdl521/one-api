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
export type ProductOffering = {
  id: 'arkclaw' | 'trae-work' | 'agent-plan'
  order: string
  name: string
  descriptionKey: string
  planKey?: string
  priceKey: string
  details: readonly string[]
  imagePath: string
  imageAltKey: string
}

export const productOfferings: readonly ProductOffering[] = [
  {
    id: 'arkclaw',
    order: '01',
    name: 'ArkClaw',
    descriptionKey: 'Deploy OpenClaw and other AI agents',
    priceKey: '¥210',
    details: ['2C4G · 40G'],
    imagePath: '/product-showcase/arkclaw.png',
    imageAltKey: 'ArkClaw compact server with AI agent nodes',
  },
  {
    id: 'trae-work',
    order: '02',
    name: 'Trae Work',
    descriptionKey: 'Team edition',
    priceKey: '¥149 / seat / month',
    details: [
      'Starting from 1 seat',
      '¥40 model allowance / seat / month',
      '30M Tokens / seat / month',
      'IDE · plugins · Solo (desktop / web)',
    ],
    imagePath: '/product-showcase/trae-work.png',
    imageAltKey: 'Trae Work developer workstation',
  },
  {
    id: 'agent-plan',
    order: '03',
    name: 'Agent Plan',
    descriptionKey: 'One key for multiple frontier models',
    planKey: 'Medium',
    priceKey: '¥200 / month',
    details: [
      '100,000 AFP monthly allowance',
      '35,000 AFP weekly allowance',
      '10,000 AFP five-hour allowance',
    ],
    imagePath: '/product-showcase/agent-plan.png',
    imageAltKey: 'Agent Plan access key and credential',
  },
]
