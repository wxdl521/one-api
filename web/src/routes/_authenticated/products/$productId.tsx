import { createFileRoute } from '@tanstack/react-router'

import { ProductDetail } from '@/features/products/product-detail'

function ProductDetailRoute() {
  const { productId } = Route.useParams()
  return <ProductDetail productId={Number(productId)} />
}

export const Route = createFileRoute('/_authenticated/products/$productId')({
  component: ProductDetailRoute,
})
