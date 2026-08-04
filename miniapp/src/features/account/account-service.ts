import { MiniAppApiError, request } from '../../lib/api'

export interface AccountOverview {
  username: string
  displayName: string
  email?: string
  quota: AccountQuota
  enabledGroups: string[]
  subscriptions: AccountSubscription[]
}

export interface AccountQuota {
  balance: number
  unit: 'quota'
}

export interface AccountSubscription {
  planTitle: string
  status: string
  endsAt: number
  quota: AccountSubscriptionQuota
}

export interface AccountSubscriptionQuota {
  remaining: number
  unlimited: boolean
  unit: 'quota'
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object'
}

function invalidAccountOverview(): MiniAppApiError {
  return new MiniAppApiError('MINIAPP_INVALID_ACCOUNT_RESPONSE', 'Mini Program account response is invalid')
}

function readQuota(value: unknown): AccountQuota {
  if (!isRecord(value) || typeof value.balance !== 'number' || !Number.isFinite(value.balance) || value.unit !== 'quota') {
    throw invalidAccountOverview()
  }
  return { balance: value.balance, unit: value.unit }
}

function readSubscriptionQuota(value: unknown): AccountSubscriptionQuota {
  if (
    !isRecord(value) ||
    typeof value.remaining !== 'number' ||
    !Number.isFinite(value.remaining) ||
    typeof value.unlimited !== 'boolean' ||
    value.unit !== 'quota'
  ) {
    throw invalidAccountOverview()
  }
  return {
    remaining: value.remaining,
    unlimited: value.unlimited,
    unit: value.unit,
  }
}

function readAccountOverview(value: unknown): AccountOverview {
  if (
    !isRecord(value) ||
    typeof value.username !== 'string' ||
    typeof value.display_name !== 'string' ||
    (value.email !== undefined && typeof value.email !== 'string') ||
    !Array.isArray(value.enabled_groups) ||
    !value.enabled_groups.every((group) => typeof group === 'string') ||
    !Array.isArray(value.subscriptions)
  ) {
    throw invalidAccountOverview()
  }

  const subscriptions = value.subscriptions.map((subscription) => {
    if (
      !isRecord(subscription) ||
      typeof subscription.plan_title !== 'string' ||
      typeof subscription.status !== 'string' ||
      typeof subscription.ends_at !== 'number' ||
      !Number.isFinite(subscription.ends_at)
    ) {
      throw invalidAccountOverview()
    }
    return {
      planTitle: subscription.plan_title,
      status: subscription.status,
      endsAt: subscription.ends_at,
      quota: readSubscriptionQuota(subscription.quota),
    }
  })

  return {
    username: value.username,
    displayName: value.display_name,
    email: value.email,
    quota: readQuota(value.quota),
    enabledGroups: value.enabled_groups,
    subscriptions,
  }
}

export async function getAccountOverview(): Promise<AccountOverview> {
  const response = await request<unknown>({
    path: '/me/overview',
    method: 'GET',
    auth: 'session',
  })
  return readAccountOverview(response)
}
