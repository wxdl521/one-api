import { createFileRoute } from '@tanstack/react-router'

import { ProductOrdersManagement } from '@/features/products/product-orders-management'

export const Route = createFileRoute('/_authenticated/products/orders/manage')({
  component: ProductOrdersManagement,
})
