import { expect, test } from 'bun:test'

import { productFormSchema } from '../lib/product-form'
import { productOrderSchema } from '../lib/product-order'
import { getOrderStatusLabel } from '../lib/product-order-status'

test('subscription product requires a subscription plan', () => {
  const result = productFormSchema.safeParse({
    name: 'Volcengine Agent Plan',
    product_type: 'subscription',
    summary: '',
    description: '',
    image_url: '',
    enabled: true,
    sort_order: 0,
  })

  expect(result.success).toBe(false)
})

test('requires a positive RMB-cent product price', () => {
  const result = productFormSchema.safeParse({
    name: 'Manual CDK',
    product_type: 'manual',
    summary: '',
    description: '',
    image_url: '',
    payment_qr_code_url: '',
    payment_instructions: '',
    price_cents: 0,
    enabled: true,
    sort_order: 0,
  })

  expect(result.success).toBe(false)
})

test('accepts QR-code payment for a product order', () => {
  expect(
    productOrderSchema.safeParse({ product_id: 1, payment_method: 'qr_code' })
      .success
  ).toBe(true)
})

test('maps a paid pending order to the manual delivery label', () => {
  expect(
    getOrderStatusLabel({
      payment_status: 'paid',
      fulfillment_status: 'pending',
    })
  ).toBe('Awaiting manual delivery')
})
