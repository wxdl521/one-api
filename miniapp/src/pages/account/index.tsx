import { Button, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { useCallback, useEffect, useState } from 'react'

import { getAccountOverview, type AccountOverview } from '../../features/account/account-service'
import { MiniAppApiError } from '../../lib/api'
import { t } from '../../i18n/strings'
import './index.scss'

function shouldReturnToLogin(error: unknown): boolean {
  return error instanceof MiniAppApiError && (
    error.status === 401 ||
    error.code === 'MINIAPP_SESSION_INVALID' ||
    error.code === 'AUTH_UNAUTHORIZED'
  )
}

function formatQuota(value: number): string {
  return value.toLocaleString('zh-CN')
}

export default function AccountPage() {
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [overview, setOverview] = useState<AccountOverview | null>(null)

  const loadOverview = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const nextOverview = await getAccountOverview()
      setOverview(nextOverview)
    } catch (loadError) {
      if (shouldReturnToLogin(loadError)) {
        setOverview(null)
        await Taro.reLaunch({ url: '/pages/index/index' })
        return
      }
      setError(t('accountLoadFailed'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadOverview()
  }, [loadOverview])

  return (
    <View className="account-shell">
      <View className="account-header">
        <Text className="account-title">{t('accountOverview')}</Text>
        <Button className="account-refresh" disabled={loading} loading={loading} onClick={() => void loadOverview()}>
          {t('refresh')}
        </Button>
      </View>

      {loading && overview === null && <Text className="account-loading">{t('accountLoading')}</Text>}
      {error !== null && <Text className="account-error">{error}</Text>}

      {overview !== null && (
        <View className="account-content">
          <View className="account-card">
            <Text className="account-name">{overview.displayName || overview.username}</Text>
            <Text className="account-username">{overview.username}</Text>
            {overview.email !== undefined && overview.email !== '' && <Text className="account-email">{overview.email}</Text>}
          </View>

          <View className="account-card">
            <Text className="account-label">{t('accountQuotaBalance')}</Text>
            <Text className="account-quota">{formatQuota(overview.quota.balance)}</Text>
          </View>

          <View className="account-card">
            <Text className="account-label">{t('accountEnabledGroups')}</Text>
            <Text className="account-value">{overview.enabledGroups.join('、')}</Text>
          </View>

          <View className="account-card">
            <Text className="account-label">{t('accountActiveSubscription')}</Text>
            {overview.subscriptions.length === 0 ? (
              <Text className="account-value">{t('accountNoSubscriptions')}</Text>
            ) : (
              overview.subscriptions.map((subscription) => (
                <View key={`${subscription.planTitle}-${subscription.endsAt}`} className="account-subscription">
                  <Text className="account-value">{subscription.planTitle}</Text>
                  <Text className="account-subscription-quota">
                    {subscription.quota.unlimited ? t('accountUnlimited') : formatQuota(subscription.quota.remaining)}
                  </Text>
                </View>
              ))
            )}
          </View>
        </View>
      )}
    </View>
  )
}
