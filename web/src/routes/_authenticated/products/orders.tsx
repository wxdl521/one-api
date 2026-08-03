import { createFileRoute } from '@tanstack/react-router'

import { ProductOrders } from '@/features/products/orders'

export const Route = createFileRoute('/_authenticated/products/orders')({
  component: ProductOrders,
})
