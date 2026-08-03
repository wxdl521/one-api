import { Button, Input, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { useState } from 'react'

import {
  getPendingIdentityTicket,
  registerWithPendingIdentity,
} from '../../features/auth/auth-service'
import { t } from '../../i18n/strings'
import './index.scss'

export default function LoginPage() {
  const [error, setError] = useState<string | null>(null)
  const [isRegistering, setIsRegistering] = useState(false)
  const [loading, setLoading] = useState(false)
  const [password, setPassword] = useState('')
  const [username, setUsername] = useState('')
  const hasPendingIdentity = getPendingIdentityTicket() !== null

  const restart = async () => {
    await Taro.reLaunch({ url: '/pages/index/index' })
  }

  const register = async () => {
    if (!hasPendingIdentity) {
      await restart()
      return
    }
    setLoading(true)
    setError(null)
    try {
      await registerWithPendingIdentity(username, password)
      await Taro.reLaunch({ url: '/pages/account/index' })
    } catch {
      setError(t('registerFailed'))
    } finally {
      setLoading(false)
    }
  }

  if (!hasPendingIdentity) {
    return (
      <View className="login-shell">
        <Text className="login-title">{t('loginRequired')}</Text>
        <Button onClick={() => void restart()}>{t('retry')}</Button>
      </View>
    )
  }

  return (
    <View className="login-shell">
      <Text className="login-title">{t('login')}</Text>
      <Text className="login-description">{t('bindingPrompt')}</Text>
      {error !== null && <Text className="login-error">{error}</Text>}
      {isRegistering ? (
        <View className="login-form">
          <Input
            aria-label={t('username')}
            placeholder={t('username')}
            value={username}
            onInput={(event) => setUsername(event.detail.value)}
          />
          <Input
            aria-label={t('password')}
            password
            placeholder={t('password')}
            value={password}
            onInput={(event) => setPassword(event.detail.value)}
          />
          <Button loading={loading} disabled={loading} onClick={() => void register()}>
            {t('register')}
          </Button>
        </View>
      ) : (
        <>
          <Button disabled={loading} onClick={() => void Taro.navigateTo({ url: '/pages/binding/index' })}>
            {t('continueBinding')}
          </Button>
          <Text className="login-description">{t('registerPrompt')}</Text>
          <Button disabled={loading} plain onClick={() => setIsRegistering(true)}>
            {t('register')}
          </Button>
        </>
      )}
    </View>
  )
}
