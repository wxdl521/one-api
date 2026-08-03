import { z } from 'zod'

export const productFormSchema = z
  .object({
    name: z.string().trim().min(1).max(128),
    summary: z.string().trim().max(255),
    description: z.string().trim(),
    image_url: z.string().trim().max(2048),
    price_cents: z.number().int().positive(),
    payment_qr_code_url: z.string().trim().max(2048),
    payment_instructions: z.string().trim(),
    product_type: z.enum(['manual', 'subscription']),
    subscription_plan_id: z.number().int().positive().nullable().optional(),
    enabled: z.boolean(),
    sort_order: z.number().int(),
  })
  .superRefine((value, ctx) => {
    if (value.product_type === 'subscription' && !value.subscription_plan_id) {
      ctx.addIssue({
        code: 'custom',
        path: ['subscription_plan_id'],
        message: 'Select subscription plan',
      })
    }
  })

export type ProductFormValues = z.infer<typeof productFormSchema>
