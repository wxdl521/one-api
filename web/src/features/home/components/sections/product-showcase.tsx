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
import {
  CodeIcon,
  Coins01Icon,
  Database01Icon,
  UserMultipleIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { productOfferings, type ProductOffering } from './product-offerings'

const accentClasses = {
  arkclaw: 'text-blue-600 dark:text-blue-300',
  'trae-work': 'text-violet-700 dark:text-violet-300',
  'agent-plan': 'text-amber-600 dark:text-amber-300',
} satisfies Record<ProductOffering['id'], string>

const traeDetailIcons = [
  UserMultipleIcon,
  Coins01Icon,
  Database01Icon,
  CodeIcon,
]

export function ProductShowcase() {
  const { t } = useTranslation()

  return (
    <main className='bg-background pt-24 pb-12 sm:pt-28 lg:pt-32 lg:pb-16'>
      <section
        className='mx-auto w-full max-w-7xl px-5 sm:px-8 lg:px-10'
        aria-labelledby='product-showcase-heading'
      >
        <header className='border-border/70 border-y py-10 text-center sm:py-14 lg:py-16'>
          <p className='text-muted-foreground mb-4 text-xs font-semibold tracking-[0.24em] uppercase'>
            {t('The One product suite')}
          </p>
          <h1
            id='product-showcase-heading'
            className='text-foreground [font-family:var(--font-serif)] text-[clamp(2.5rem,5.5vw,5.25rem)] leading-[1.08] font-medium tracking-[-0.045em]'
          >
            {t('Three ways to make AI work for you')}
          </h1>
        </header>

        <div className='divide-border/70 grid divide-y lg:grid-cols-3 lg:divide-x lg:divide-y-0'>
          {productOfferings.map((offering, offeringIndex) => {
            const detailIcons =
              offering.id === 'trae-work' ? traeDetailIcons : undefined
            const accentClassName = accentClasses[offering.id]

            return (
              <article
                key={offering.id}
                className='flex min-w-0 flex-col px-0 py-9 sm:py-11 lg:px-8 lg:py-12 xl:px-10'
                aria-labelledby={`${offering.id}-title`}
              >
                <div className={`flex items-center gap-3 ${accentClassName}`}>
                  <span className='text-sm font-medium tabular-nums'>
                    {offering.order}
                  </span>
                  <div className='h-px flex-1 bg-current opacity-30' />
                </div>

                <h2
                  id={`${offering.id}-title`}
                  className={`mt-5 [font-family:var(--font-serif)] text-[clamp(3rem,5vw,4.75rem)] leading-[0.95] font-medium tracking-[-0.07em] ${accentClassName}`}
                >
                  {offering.name}
                </h2>
                <p className='text-foreground mt-4 text-base leading-relaxed font-medium sm:text-lg'>
                  {t(offering.descriptionKey)}
                </p>

                <div className={`mt-6 border-t pt-6 ${accentClassName}`}>
                  {offering.planKey && (
                    <p className='text-base font-semibold tracking-wide'>
                      {t(offering.planKey)}
                    </p>
                  )}
                  <p className='mt-2 [font-family:var(--font-serif)] text-4xl leading-none font-medium tracking-[-0.05em] sm:text-5xl'>
                    {t(offering.priceKey)}
                  </p>
                </div>

                <ul className='text-foreground mt-5 space-y-3 text-sm leading-relaxed sm:text-[15px]'>
                  {offering.details.map((detail, detailIndex) => {
                    const DetailIcon = detailIcons?.[detailIndex]

                    return (
                      <li key={detail} className='flex items-start gap-2.5'>
                        {DetailIcon && (
                          <HugeiconsIcon
                            icon={DetailIcon}
                            className={`mt-0.5 size-4 shrink-0 ${accentClassName}`}
                            strokeWidth={1.8}
                            aria-hidden='true'
                          />
                        )}
                        <span>{t(detail)}</span>
                      </li>
                    )
                  })}
                </ul>

                <figure className='bg-muted/35 border-border/60 mt-8 aspect-[2/3] overflow-hidden rounded-xl border'>
                  <img
                    src={offering.imagePath}
                    alt={t(offering.imageAltKey)}
                    className='size-full object-cover object-center'
                    loading={offeringIndex === 0 ? 'eager' : 'lazy'}
                    fetchPriority={offeringIndex === 0 ? 'high' : 'auto'}
                  />
                </figure>
              </article>
            )
          })}
        </div>
      </section>
    </main>
  )
}
