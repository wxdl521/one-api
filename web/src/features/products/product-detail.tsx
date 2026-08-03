import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'

import { createProductOrder, getProduct } from './api'
import type { ProductOrder } from './types'

export function ProductDetail(props: { productId: number }) {
  const { t } = useTranslation()
  const [qrOrder, setQrOrder] = useState<ProductOrder | null>(null)
  const { data, isLoading } = useQuery({
    queryKey: ['product', props.productId],
    queryFn: () => getProduct(props.productId),
  })
  const product = data?.data
  let content: ReactNode
  if (isLoading) {
    content = <p>{t('Loading...')}</p>
  } else if (!product) {
    content = <p>{t('Product not found')}</p>
  } else {
    const purchaseAction =
      product.product_type === 'subscription' ? (
        <Button render={<Link to='/subscriptions' />}>
          {t('Manage subscriptions')}
        </Button>
      ) : (
        <ProductPurchaseActions
          productId={product.id}
          onQRCodeOrder={setQrOrder}
        />
      )
    content = (
      <article className='mx-auto max-w-3xl space-y-6'>
        {product.image_url ? (
          <img
            src={product.image_url}
            alt=''
            className='max-h-96 w-full rounded-lg object-cover'
          />
        ) : null}
        <p className='text-muted-foreground text-lg'>{product.summary}</p>
        <p className='text-2xl font-semibold'>
          ¥{(product.price_cents / 100).toFixed(2)}
        </p>
        <p className='whitespace-pre-wrap'>{product.description}</p>
        {purchaseAction}
        {qrOrder?.payment_qr_code_url ? (
          <section className='space-y-3 rounded-lg border p-4'>
            <h2 className='font-semibold'>{t('Scan to pay')}</h2>
            <img
              src={qrOrder.payment_qr_code_url}
              alt={t('Payment QR code')}
              className='size-52 rounded-lg object-contain'
            />
            <p className='text-muted-foreground text-sm whitespace-pre-wrap'>
              {qrOrder.payment_instructions}
            </p>
          </section>
        ) : null}
      </article>
    )
  }
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {product?.name ?? t('Product details')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>{content}</SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function ProductPurchaseActions(props: {
  productId: number
  onQRCodeOrder: (order: ProductOrder) => void
}) {
  const { t } = useTranslation()
  const walletOrder = async () => {
    await createProductOrder({
      product_id: props.productId,
      payment_method: 'wallet',
    })
  }
  const qrCodeOrder = async () => {
    const response = await createProductOrder({
      product_id: props.productId,
      payment_method: 'qr_code',
    })
    if (response.success && response.data) props.onQRCodeOrder(response.data)
  }

  return (
    <div className='flex flex-wrap gap-3'>
      <Button onClick={() => void walletOrder()}>{t('Pay with wallet')}</Button>
      <Button variant='outline' onClick={() => void qrCodeOrder()}>
        {t('Pay by QR code')}
      </Button>
    </div>
  )
}
