import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type ChangeEvent } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  createProduct,
  getAdminProducts,
  updateProduct,
  updateProductStatus,
  uploadProductImage,
} from './api'
import { productFormSchema, type ProductFormValues } from './lib/product-form'
import type { Product, ProductPayload } from './types'

const defaultValues: ProductFormValues = {
  name: '',
  summary: '',
  description: '',
  image_url: '',
  price_cents: 100,
  payment_qr_code_url: '',
  payment_instructions: '',
  product_type: 'manual',
  subscription_plan_id: null,
  enabled: true,
  sort_order: 0,
}

export function ProductManagement() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState<Product | null>(null)
  const { data } = useQuery({
    queryKey: ['admin-products'],
    queryFn: getAdminProducts,
  })
  const form = useForm<ProductFormValues>({
    resolver: zodResolver(productFormSchema),
    defaultValues,
  })
  const save = useMutation({
    mutationFn: async (values: ProductFormValues) => {
      const payload: ProductPayload = {
        ...values,
        subscription_plan_id:
          values.product_type === 'subscription'
            ? (values.subscription_plan_id ?? null)
            : null,
      }
      return editing
        ? updateProduct(editing.id, payload)
        : createProduct(payload)
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['admin-products'] }),
        queryClient.invalidateQueries({ queryKey: ['products'] }),
      ])
      form.reset(defaultValues)
      setEditing(null)
      toast.success(t('Product saved'))
    },
  })
  const changeStatus = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) =>
      updateProductStatus(id, enabled),
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: ['admin-products'] }),
        queryClient.invalidateQueries({ queryKey: ['products'] }),
      ]),
  })
  const products = data?.data ?? []

  const uploadImage = async (
    event: ChangeEvent<HTMLInputElement>,
    field: 'image_url' | 'payment_qr_code_url'
  ) => {
    const file = event.target.files?.[0]
    if (!file) return
    const response = await uploadProductImage(file)
    if (!response.success || !response.data?.url) return
    form.setValue(field, response.data.url, { shouldDirty: true })
    toast.success(t('Image uploaded'))
  }

  const startEdit = (product: Product) => {
    setEditing(product)
    form.reset(product)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Product Management')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          onClick={() => {
            setEditing(null)
            form.reset(defaultValues)
          }}
        >
          {t('Create Product')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='grid gap-6 lg:grid-cols-[minmax(0,1fr)_24rem]'>
          <div className='space-y-3'>
            {products.map((product) => (
              <div
                key={product.id}
                className='flex items-center gap-3 rounded-lg border p-3'
              >
                <div className='min-w-0 flex-1'>
                  <p className='font-medium'>{product.name}</p>
                  <p className='text-muted-foreground text-sm'>
                    {product.product_type === 'subscription'
                      ? t('Subscription')
                      : t('Manual delivery')}
                  </p>
                </div>
                <Switch
                  checked={product.enabled}
                  onCheckedChange={(enabled) =>
                    changeStatus.mutate({ id: product.id, enabled })
                  }
                  aria-label={t('Publish product')}
                />
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => startEdit(product)}
                >
                  {t('Edit Product')}
                </Button>
              </div>
            ))}
          </div>
          <form
            className='space-y-3 rounded-lg border p-4'
            onSubmit={form.handleSubmit((values) => save.mutate(values))}
          >
            <h2 className='font-semibold'>
              {editing ? t('Edit Product') : t('Create Product')}
            </h2>
            <Input {...form.register('name')} placeholder={t('Name')} />
            <Input
              {...form.register('summary')}
              placeholder={t('Product summary')}
            />
            <Input
              {...form.register('image_url')}
              placeholder={t('Cover image URL')}
            />
            <Input
              accept='image/png,image/jpeg,image/webp,image/gif'
              type='file'
              onChange={(event) => void uploadImage(event, 'image_url')}
            />
            <Input
              type='number'
              min={1}
              {...form.register('price_cents', { valueAsNumber: true })}
              placeholder={t('Price in CNY cents')}
            />
            <Input
              {...form.register('payment_qr_code_url')}
              placeholder={t('Payment QR code URL')}
            />
            <Input
              accept='image/png,image/jpeg,image/webp,image/gif'
              type='file'
              onChange={(event) =>
                void uploadImage(event, 'payment_qr_code_url')
              }
            />
            <Textarea
              {...form.register('payment_instructions')}
              placeholder={t('Payment instructions')}
            />
            <Textarea
              {...form.register('description')}
              placeholder={t('Product description')}
            />
            <label className='block text-sm'>
              {t('Product type')}
              <select
                className='mt-1 h-8 w-full rounded-lg border bg-transparent px-2'
                {...form.register('product_type')}
              >
                <option value='manual'>{t('Manual delivery')}</option>
                <option value='subscription'>{t('Subscription')}</option>
              </select>
            </label>
            {form.watch('product_type') === 'subscription' ? (
              <Input
                type='number'
                {...form.register('subscription_plan_id', {
                  setValueAs: (value) => (value ? Number(value) : null),
                })}
                placeholder={t('Subscription plan ID')}
              />
            ) : null}
            <Input
              type='number'
              {...form.register('sort_order', { valueAsNumber: true })}
              placeholder={t('Sort order')}
            />
            <label className='flex items-center gap-2 text-sm'>
              <Switch
                checked={form.watch('enabled')}
                onCheckedChange={(enabled) => form.setValue('enabled', enabled)}
              />
              {t('Publish product')}
            </label>
            <Button type='submit' disabled={save.isPending}>
              {save.isPending ? t('Saving...') : t('Save changes')}
            </Button>
          </form>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
