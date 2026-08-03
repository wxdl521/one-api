import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'

import { getMyProductOrders } from './api'
import { getOrderStatusLabel } from './lib/product-order-status'

export function ProductOrders() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['product-orders'],
    queryFn: getMyProductOrders,
  })
  const orders = data?.data ?? []

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('My orders')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {isLoading ? <p>{t('Loading...')}</p> : null}
        {!isLoading && orders.length === 0 ? (
          <p className='text-muted-foreground'>{t('No orders yet')}</p>
        ) : null}
        <div className='space-y-3'>
          {orders.map((order) => (
            <article key={order.id} className='rounded-lg border p-4'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <h2 className='font-semibold'>{order.product_name}</h2>
                <span className='text-muted-foreground text-sm'>
                  {t(getOrderStatusLabel(order))}
                </span>
              </div>
              <p className='mt-2'>¥{(order.price_cents / 100).toFixed(2)}</p>
              <p className='text-muted-foreground mt-1 text-sm'>
                {t('Order number')}: {order.id}
              </p>
            </article>
          ))}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
