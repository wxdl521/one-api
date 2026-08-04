import { Button, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { useCallback, useEffect, useState } from 'react'

import {
  getMiniAppCommerce,
  setMiniAppCheckoutWebURL,
  startMiniAppCheckout,
  type MiniAppCommerce,
} from '../../features/commerce/commerce-service'
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

function formatPrice(value: number, currency: string): string {
  return `${currency} ${value.toFixed(2)}`
}

export default function ProductsPage() {
  const [commerce, setCommerce] = useState<MiniAppCommerce | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [checkoutID, setCheckoutID] = useState<string | null>(null)

  const loadCommerce = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setCommerce(await getMiniAppCommerce())
    } catch (loadError) {
      if (shouldReturnToLogin(loadError)) {
        setCommerce(null)
        await Taro.reLaunch({ url: '/pages/index/index' })
        return
      }
      setError(t('commerceLoadFailed'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadCommerce()
  }, [loadCommerce])

  const continueCheckout = async (targetType: 'plan' | 'product', targetID: number) => {
    const identifier = `${targetType}-${targetID}`
    setCheckoutID(identifier)
    setError(null)
    try {
      setMiniAppCheckoutWebURL(await startMiniAppCheckout(targetType, targetID))
      await Taro.navigateTo({ url: '/pages/checkout/index' })
    } catch (checkoutError) {
      if (shouldReturnToLogin(checkoutError)) {
        await Taro.reLaunch({ url: '/pages/index/index' })
        return
      }
      setError(t('checkoutFailed'))
    } finally {
      setCheckoutID(null)
    }
  }

  return (
    <View className="commerce-shell">
      <View className="commerce-header">
        <Text className="commerce-title">{t('services')}</Text>
        <Button disabled={loading} loading={loading} onClick={() => void loadCommerce()}>{t('refresh')}</Button>
      </View>
      {error !== null && <Text className="commerce-error">{error}</Text>}
      {loading && commerce === null && <Text>{t('refresh')}</Text>}
      {commerce !== null && (
        <>
          <Text className="commerce-section-title">{t('plans')}</Text>
          {commerce.plans.length === 0 && <Text>{t('noPlans')}</Text>}
          {commerce.plans.map((plan) => (
            <View key={plan.id} className="commerce-card">
              <Text className="commerce-card-title">{plan.title}</Text>
              <Text>{plan.subtitle}</Text>
              <Text>{formatPrice(plan.priceAmount, plan.currency)} / {plan.durationValue} {plan.durationUnit}</Text>
              <Button
                disabled={checkoutID !== null}
                loading={checkoutID === `plan-${plan.id}`}
                onClick={() => void continueCheckout('plan', plan.id)}
              >
                {t('checkoutOpen')}
              </Button>
            </View>
          ))}
          <Text className="commerce-section-title">{t('products')}</Text>
          {commerce.products.length === 0 && <Text>{t('noProducts')}</Text>}
          {commerce.products.map((product) => (
            <View key={product.id} className="commerce-card">
              <Text className="commerce-card-title">{product.name}</Text>
              <Text>{product.summary}</Text>
              <Text>{product.description}</Text>
              <Text>{formatPrice(product.priceCents / 100, 'USD')}</Text>
              <Button
                disabled={checkoutID !== null}
                loading={checkoutID === `product-${product.id}`}
                onClick={() => void continueCheckout('product', product.id)}
              >
                {t('checkoutOpen')}
              </Button>
            </View>
          ))}
        </>
      )}
    </View>
  )
}
