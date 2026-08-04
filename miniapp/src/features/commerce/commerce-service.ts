import { MiniAppApiError, request } from '../../lib/api'

export interface MiniAppPlan {
  id: number
  title: string
  subtitle: string
  priceAmount: number
  currency: string
  durationUnit: string
  durationValue: number
}

export interface MiniAppProduct {
  id: number
  name: string
  summary: string
  description: string
  imageUrl: string
  priceCents: number
  productType: 'manual' | 'subscription'
}

export interface MiniAppOrder {
  id: number
  productName: string
  priceCents: number
  paymentStatus: string
  fulfillmentStatus: string
  createdAt: number
}

export interface MiniAppCommerce {
  plans: MiniAppPlan[]
  products: MiniAppProduct[]
  orders: MiniAppOrder[]
}

type CheckoutTargetType = 'plan' | 'product'

let checkoutWebURL: string | null = null

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object'
}

function invalidCommerceResponse(): MiniAppApiError {
  return new MiniAppApiError('MINIAPP_INVALID_COMMERCE_RESPONSE', 'Mini Program commerce response is invalid')
}

function readPositiveInteger(value: unknown): number {
  if (typeof value !== 'number' || !Number.isInteger(value) || value <= 0) {
    throw invalidCommerceResponse()
  }
  return value
}

function readFiniteNumber(value: unknown): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw invalidCommerceResponse()
  }
  return value
}

function readString(value: unknown): string {
  if (typeof value !== 'string') {
    throw invalidCommerceResponse()
  }
  return value
}

function readPlan(value: unknown): MiniAppPlan {
  if (!isRecord(value)) {
    throw invalidCommerceResponse()
  }
  return {
    id: readPositiveInteger(value.id),
    title: readString(value.title),
    subtitle: readString(value.subtitle),
    priceAmount: readFiniteNumber(value.price_amount),
    currency: readString(value.currency),
    durationUnit: readString(value.duration_unit),
    durationValue: readPositiveInteger(value.duration_value),
  }
}

function readProduct(value: unknown): MiniAppProduct {
  if (!isRecord(value) || (value.product_type !== 'manual' && value.product_type !== 'subscription')) {
    throw invalidCommerceResponse()
  }
  return {
    id: readPositiveInteger(value.id),
    name: readString(value.name),
    summary: readString(value.summary),
    description: readString(value.description),
    imageUrl: readString(value.image_url),
    priceCents: readPositiveInteger(value.price_cents),
    productType: value.product_type,
  }
}

function readOrder(value: unknown): MiniAppOrder {
  if (!isRecord(value)) {
    throw invalidCommerceResponse()
  }
  return {
    id: readPositiveInteger(value.id),
    productName: readString(value.product_name),
    priceCents: readPositiveInteger(value.price_cents),
    paymentStatus: readString(value.payment_status),
    fulfillmentStatus: readString(value.fulfillment_status),
    createdAt: readFiniteNumber(value.created_at),
  }
}

function readList(value: unknown, itemReader: (item: unknown) => MiniAppPlan): MiniAppPlan[]
function readList(value: unknown, itemReader: (item: unknown) => MiniAppProduct): MiniAppProduct[]
function readList(value: unknown, itemReader: (item: unknown) => MiniAppOrder): MiniAppOrder[]
function readList<T>(value: unknown, itemReader: (item: unknown) => T): T[] {
  if (!Array.isArray(value)) {
    throw invalidCommerceResponse()
  }
  return value.map(itemReader)
}

function trustedBusinessOrigin(): URL {
  const configuredOrigin = typeof __MINIAPP_BINDING_ORIGIN__ === 'string'
    ? __MINIAPP_BINDING_ORIGIN__.trim()
    : ''
  try {
    const origin = new URL(configuredOrigin)
    if (
      origin.protocol !== 'https:' ||
      origin.hostname === '' ||
      origin.username !== '' ||
      origin.password !== '' ||
      origin.pathname !== '/' ||
      origin.search !== '' ||
      origin.hash !== ''
    ) {
      throw new Error('invalid business origin')
    }
    return origin
  } catch {
    throw invalidCommerceResponse()
  }
}

function readCheckoutURL(value: unknown): string {
  if (!isRecord(value) || typeof value.checkout_url !== 'string') {
    throw invalidCommerceResponse()
  }
  try {
    const trustedOrigin = trustedBusinessOrigin()
    const checkoutURL = new URL(value.checkout_url)
    const fragment = new URLSearchParams(checkoutURL.hash.slice(1))
    const tickets = fragment.getAll('checkout_ticket')
    if (
      checkoutURL.origin !== trustedOrigin.origin ||
      checkoutURL.username !== '' ||
      checkoutURL.password !== '' ||
      checkoutURL.pathname !== '/miniapp-checkout' ||
      checkoutURL.search !== '' ||
      tickets.length !== 1 ||
      fragment.size !== 1 ||
      tickets[0].trim() === '' ||
      tickets[0].length > 512
    ) {
      throw new Error('invalid checkout URL')
    }
    return checkoutURL.toString()
  } catch (error) {
    if (error instanceof MiniAppApiError) {
      throw error
    }
    throw invalidCommerceResponse()
  }
}

export async function getMiniAppCommerce(): Promise<MiniAppCommerce> {
  const [plans, products, orders] = await Promise.all([
    request<unknown>({ path: '/plans', method: 'GET', auth: 'session' }),
    request<unknown>({ path: '/products', method: 'GET', auth: 'session' }),
    request<unknown>({ path: '/orders', method: 'GET', auth: 'session' }),
  ])
  return {
    plans: readList(plans, readPlan),
    products: readList(products, readProduct),
    orders: readList(orders, readOrder),
  }
}

export async function startMiniAppCheckout(targetType: CheckoutTargetType, targetID: number): Promise<string> {
  if (!Number.isInteger(targetID) || targetID <= 0) {
    throw invalidCommerceResponse()
  }
  const response = await request<unknown, { target_type: CheckoutTargetType; target_id: number }>({
    path: '/checkout',
    method: 'POST',
    data: { target_type: targetType, target_id: targetID },
    auth: 'session',
  })
  return readCheckoutURL(response)
}

export function setMiniAppCheckoutWebURL(url: string): void {
  checkoutWebURL = url
}

export function getMiniAppCheckoutWebURL(): string | null {
  return checkoutWebURL
}

export function clearMiniAppCheckoutWebURL(): void {
  checkoutWebURL = null
}
