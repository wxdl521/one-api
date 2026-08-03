export function getOrderStatusLabel(order: {
  payment_status: string
  fulfillment_status: string
}): string {
  if (order.fulfillment_status === 'cancelled') return 'Cancelled'
  if (order.payment_status === 'pending') return 'Awaiting payment confirmation'
  if (order.fulfillment_status === 'shipped') return 'Shipped'
  return 'Awaiting manual delivery'
}
