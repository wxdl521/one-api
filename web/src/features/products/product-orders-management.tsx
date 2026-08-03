import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'

import {
  cancelProductOrder,
  confirmProductOrder,
  getAdminProductOrders,
  shipProductOrder,
} from './api'
import { getOrderStatusLabel } from './lib/product-order-status'

export function ProductOrdersManagement() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['admin-product-orders'],
    queryFn: getAdminProductOrders,
  })
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ['admin-product-orders'] })
  const confirm = useMutation({
    mutationFn: confirmProductOrder,
    onSuccess: refresh,
  })
  const ship = useMutation({ mutationFn: shipProductOrder, onSuccess: refresh })
  const cancel = useMutation({
    mutationFn: cancelProductOrder,
    onSuccess: refresh,
  })
  const orders = data?.data ?? []

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Product Orders')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {isLoading ? <p>{t('Loading...')}</p> : null}
        <div className='space-y-3'>
          {orders.map((order) => (
            <article key={order.id} className='rounded-lg border p-4'>
              <div className='flex flex-wrap items-center justify-between gap-3'>
                <div>
                  <h2 className='font-semibold'>{order.product_name}</h2>
                  <p className='text-muted-foreground text-sm'>
                    {t('Order number')}: {order.id} · ¥
                    {(order.price_cents / 100).toFixed(2)} ·{' '}
                    {t(getOrderStatusLabel(order))}
                  </p>
                </div>
                <div className='flex flex-wrap gap-2'>
                  <Button
                    size='sm'
                    disabled={order.payment_status !== 'pending'}
                    onClick={() => confirm.mutate(order.id)}
                  >
                    {t('Confirm payment')}
                  </Button>
                  <Button
                    size='sm'
                    disabled={
                      order.payment_status !== 'paid' ||
                      order.fulfillment_status !== 'pending'
                    }
                    onClick={() => ship.mutate(order.id)}
                  >
                    {t('Mark as shipped')}
                  </Button>
                  <Button
                    size='sm'
                    variant='outline'
                    disabled={order.fulfillment_status !== 'pending'}
                    onClick={() => cancel.mutate(order.id)}
                  >
                    {t('Cancel order')}
                  </Button>
                </div>
              </div>
            </article>
          ))}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
