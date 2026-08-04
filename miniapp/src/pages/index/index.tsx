import { Button, Text, View } from '@tarojs/components'
import Taro, { useDidShow } from '@tarojs/taro'
import { useCallback, useRef, useState } from 'react'

import { loginWithWechat } from '../../features/auth/auth-service'
import { t } from '../../i18n/strings'
import './index.scss'

export default function IndexPage() {
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const isLoggingIn = useRef(false)

  const connect = useCallback(async () => {
    if (isLoggingIn.current) {
      return
    }
    isLoggingIn.current = true
    setLoading(true)
    setError(null)
    try {
      const outcome = await loginWithWechat()
      await Taro.reLaunch({
        url: outcome.kind === 'authenticated' ? '/pages/account/index' : '/pages/login/index',
      })
    } catch {
      setError(t('loginFailed'))
    } finally {
      isLoggingIn.current = false
      setLoading(false)
    }
  }, [])

  useDidShow(() => {
    void connect()
  })

  return (
    <View className="status-shell">
      <Text className="status-title">{t('connecting')}</Text>
      {error !== null && <Text className="status-error">{error}</Text>}
      {error !== null && (
        <Button loading={loading} disabled={loading} onClick={() => void connect()}>
          {t('retry')}
        </Button>
      )}
    </View>
  )
}
