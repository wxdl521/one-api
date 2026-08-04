import { Button, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { useCallback, useEffect, useState } from 'react'

import { getMiniAppCommerce, type MiniAppOrder } from '../../features/commerce/commerce-service'
import { t } from '../../i18n/strings'
import { MiniAppApiError } from '../../lib/api'

function shouldReturnToLogin(error: unknown): boolean {
  return error instanceof MiniAppApiError && (
    error.status === 401 ||
    error.code === 'MINIAPP_SESSION_INVALID' ||
    error.code === 'MINIAPP_SESSION_UNAVAILABLE' ||
    error.code === 'AUTH_UNAUTHORIZED'
  )
}

function formatOrderTime(value: number): string {
  return value > 0 ? new Date(value * 1000).toLocaleString('zh-CN') : '-'
}

export default function OrdersPage() {
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [orders, setOrders] = useState<MiniAppOrder[]>([])

  const loadOrders = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const commerce = await getMiniAppCommerce()
      setOrders(commerce.orders)
    } catch (loadError) {
      if (shouldReturnToLogin(loadError)) {
        setOrders([])
        await Taro.reLaunch({ url: '/pages/index/index' })
        return
      }
      setError(t('commerceLoadFailed'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadOrders()
  }, [loadOrders])

  return (
    <View className="commerce-shell">
      <View className="commerce-header">
        <Text className="commerce-title">{t('orders')}</Text>
        <Button disabled={loading} loading={loading} onClick={() => void loadOrders()}>{t('refresh')}</Button>
      </View>
      {error !== null && <Text className="commerce-error">{error}</Text>}
      {!loading && orders.length === 0 && <Text>{t('noOrders')}</Text>}
      {orders.map((order) => (
        <View key={order.id} className="commerce-card">
          <Text className="commerce-card-title">{order.productName}</Text>
          <Text>USD {(order.priceCents / 100).toFixed(2)}</Text>
          <Text>{order.paymentStatus} · {order.fulfillmentStatus}</Text>
          <Text>{formatOrderTime(order.createdAt)}</Text>
        </View>
      ))}
    </View>
  )
}
