import { api } from '@/lib/api'

import type {
  Product,
  ProductOrder,
  ProductOrderPayload,
  ProductPayload,
} from './types'

type ApiResponse<T> = { success: boolean; data?: T; message?: string }

export async function getProducts(): Promise<ApiResponse<Product[]>> {
  return (await api.get('/api/product')).data
}
export async function getProduct(id: number): Promise<ApiResponse<Product>> {
  return (await api.get(`/api/product/${id}`)).data
}
export async function getAdminProducts(): Promise<ApiResponse<Product[]>> {
  return (await api.get('/api/product/admin')).data
}
export async function createProduct(
  data: ProductPayload
): Promise<ApiResponse<Product>> {
  return (await api.post('/api/product/admin', data)).data
}
export async function updateProduct(
  id: number,
  data: ProductPayload
): Promise<ApiResponse<void>> {
  return (await api.put(`/api/product/admin/${id}`, data)).data
}
export async function updateProductStatus(
  id: number,
  enabled: boolean
): Promise<ApiResponse<void>> {
  return (await api.patch(`/api/product/admin/${id}/status`, { enabled })).data
}

export async function uploadProductImage(
  file: File
): Promise<ApiResponse<{ url: string }>> {
  const data = new FormData()
  data.append('file', file)
  return (await api.post('/api/product/admin/upload', data)).data
}

export async function createProductOrder(
  data: ProductOrderPayload
): Promise<ApiResponse<ProductOrder>> {
  return (await api.post('/api/product/orders', data)).data
}

export async function getMyProductOrders(): Promise<
  ApiResponse<ProductOrder[]>
> {
  return (await api.get('/api/product/orders/self')).data
}

export async function getAdminProductOrders(): Promise<
  ApiResponse<ProductOrder[]>
> {
  return (await api.get('/api/product/admin/orders')).data
}

export async function confirmProductOrder(
  id: number
): Promise<ApiResponse<void>> {
  return (await api.patch(`/api/product/admin/orders/${id}/confirm`)).data
}

export async function shipProductOrder(id: number): Promise<ApiResponse<void>> {
  return (await api.patch(`/api/product/admin/orders/${id}/ship`)).data
}

export async function cancelProductOrder(
  id: number
): Promise<ApiResponse<void>> {
  return (await api.patch(`/api/product/admin/orders/${id}/cancel`)).data
}
