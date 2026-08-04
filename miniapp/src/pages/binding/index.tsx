import { Button, Text, View, WebView } from '@tarojs/components'
import Taro, { useDidHide, useDidShow, useUnload } from '@tarojs/taro'
import { useCallback, useRef, useState } from 'react'

import {
  clearPendingIdentityTicket,
  createBinding,
  getBindingStatus,
  loginWithWechat,
} from '../../features/auth/auth-service'
import { t } from '../../i18n/strings'
import { BindingLifecycle, type BindingFailureKey } from './lifecycle'
import './index.scss'

export default function BindingPage() {
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [webUrl, setWebUrl] = useState<string | null>(null)
  const authAttemptRef = useRef(0)
  const completeBindingRef = useRef<() => void>(() => undefined)
  const isMountedRef = useRef(true)
  const lifecycleRef = useRef<BindingLifecycle | null>(null)

  const completeBinding = useCallback(async () => {
    const attempt = authAttemptRef.current
    setLoading(true)
    try {
      const outcome = await loginWithWechat()
      if (!isMountedRef.current || attempt !== authAttemptRef.current) {
        return
      }
      if (outcome.kind !== 'authenticated') {
        throw new Error('binding did not create a normal session')
      }
      await Taro.reLaunch({ url: '/pages/account/index' })
    } catch {
      if (!isMountedRef.current || attempt !== authAttemptRef.current) {
        return
      }
      setWebUrl(null)
      setError(t('bindingFailed'))
      setLoading(false)
    }
  }, [])
  completeBindingRef.current = () => {
    void completeBinding()
  }

  if (lifecycleRef.current === null) {
    lifecycleRef.current = new BindingLifecycle({
      createBinding,
      getStatus: getBindingStatus,
      onBound: () => completeBindingRef.current(),
      onError: (key: BindingFailureKey) => {
        setWebUrl(null)
        setLoading(false)
        setError(t(key))
      },
      onLoading: () => {
        setLoading(true)
        setError(null)
      },
      onReady: (nextWebUrl) => {
        setWebUrl(nextWebUrl)
        setLoading(false)
      },
    })
  }

  const restartAuthentication = useCallback(async () => {
    authAttemptRef.current += 1
    lifecycleRef.current?.cancel()
    clearPendingIdentityTicket()
    await Taro.reLaunch({ url: '/pages/index/index' })
  }, [])

  useDidShow(() => {
    void lifecycleRef.current?.show()
  })

  useDidHide(() => {
    lifecycleRef.current?.hide()
  })

  useUnload(() => {
    isMountedRef.current = false
    authAttemptRef.current += 1
    lifecycleRef.current?.unload()
  })

  if (webUrl !== null) {
    return <WebView src={webUrl} onError={() => lifecycleRef.current?.webViewFailed()} />
  }

  return (
    <View className="binding-shell">
      <Text className="binding-title">{t('binding')}</Text>
      {loading && <Text className="binding-description">{t('bindingLoading')}</Text>}
      {error !== null && <Text className="binding-error">{error}</Text>}
      {error !== null && (
        <Button onClick={() => void restartAuthentication()}>{t('retry')}</Button>
      )}
    </View>
  )
}
