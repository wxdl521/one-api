import { z } from 'zod'

export const productSchema = z.object({
  id: z.number(),
  name: z.string(),
  summary: z.string(),
  description: z.string(),
  image_url: z.string(),
  price_cents: z.number().int().positive(),
  payment_qr_code_url: z.string(),
  payment_instructions: z.string(),
  product_type: z.enum(['manual', 'subscription']),
  subscription_plan_id: z.number().nullable().optional(),
  enabled: z.boolean(),
  sort_order: z.number(),
})

export type Product = z.infer<typeof productSchema>
export type ProductPayload = Omit<Product, 'id'>

export const productOrderSchema = z.object({
  product_id: z.number().int().positive(),
  payment_method: z.enum(['wallet', 'qr_code']),
})

export const productOrderSchemaResponse = z.object({
  id: z.number(),
  user_id: z.number(),
  product_id: z.number(),
  product_name: z.string(),
  price_cents: z.number(),
  payment_method: z.enum(['wallet', 'qr_code']),
  payment_status: z.enum(['pending', 'paid']),
  fulfillment_status: z.enum(['pending', 'shipped', 'cancelled']),
  paid_quota: z.number(),
  payment_qr_code_url: z.string().optional(),
  payment_instructions: z.string().optional(),
})

export type ProductOrderPayload = z.infer<typeof productOrderSchema>
export type ProductOrder = z.infer<typeof productOrderSchemaResponse>
