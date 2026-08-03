import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { PackageOpen } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

import { getProducts } from './api'

export function Products() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['products'],
    queryFn: getProducts,
  })
  const products = data?.data ?? []
  let content: ReactNode
  if (isLoading) {
    content = <p>{t('Loading...')}</p>
  } else if (products.length === 0) {
    content = (
      <div className='text-muted-foreground flex min-h-72 flex-col items-center justify-center gap-3'>
        <PackageOpen className='size-10' />
        <p>{t('No products available')}</p>
      </div>
    )
  } else {
    content = (
      <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
        {products.map((product) => (
          <Card key={product.id} className='overflow-hidden'>
            {product.image_url ? (
              <img
                src={product.image_url}
                alt=''
                className='h-40 w-full object-cover'
              />
            ) : null}
            <CardHeader>
              <div className='flex items-start justify-between gap-3'>
                <CardTitle>{product.name}</CardTitle>
                <Badge variant='secondary'>
                  {product.product_type === 'subscription'
                    ? t('Subscription')
                    : t('Manual delivery')}
                </Badge>
              </div>
              <CardDescription>{product.summary}</CardDescription>
            </CardHeader>
            <CardContent className='text-muted-foreground line-clamp-3 text-sm'>
              {product.description}
            </CardContent>
            <CardFooter>
              <div className='flex w-full items-center justify-between gap-3'>
                <span className='font-semibold'>
                  ¥{(product.price_cents / 100).toFixed(2)}
                </span>
                <Button
                  variant='outline'
                  render={
                    <Link
                      to='/products/$productId'
                      params={{ productId: String(product.id) }}
                    />
                  }
                >
                  {t('View details')}
                </Button>
              </div>
            </CardFooter>
          </Card>
        ))}
      </div>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Products')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>{content}</SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
