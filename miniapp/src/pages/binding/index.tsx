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
import './index.scss'

const bindingPollIntervalMs = 3_000
const bindingTimeoutMs = 5 * 60 * 1_000

export default function BindingPage() {
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [webUrl, setWebUrl] = useState<string | null>(null)
  const bindingIdRef = useRef<string | null>(null)
  const deadlineRef = useRef(0)
  const foregroundRef = useRef(false)
  const hasStartedRef = useRef(false)
  const isCompletingRef = useRef(false)
  const isExpiredRef = useRef(false)
  const isPollInFlightRef = useRef(false)
  const isStoppedRef = useRef(false)
  const deadlineTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const stopPolling = useCallback(() => {
    if (pollTimerRef.current !== null) {
      clearInterval(pollTimerRef.current)
      pollTimerRef.current = null
    }
  }, [])

  const stopDeadline = useCallback(() => {
    if (deadlineTimerRef.current !== null) {
      clearTimeout(deadlineTimerRef.current)
      deadlineTimerRef.current = null
    }
  }, [])

  const expireBinding = useCallback(() => {
    if (isExpiredRef.current || isStoppedRef.current) {
      return
    }
    isExpiredRef.current = true
    isStoppedRef.current = true
    stopPolling()
    stopDeadline()
    setWebUrl(null)
    setLoading(false)
    setError(t('bindingExpired'))
  }, [stopDeadline, stopPolling])

  const restartAuthentication = useCallback(async () => {
    isStoppedRef.current = true
    stopPolling()
    stopDeadline()
    clearPendingIdentityTicket()
    await Taro.reLaunch({ url: '/pages/index/index' })
  }, [stopDeadline, stopPolling])

  const completeBinding = useCallback(async () => {
    if (isCompletingRef.current) {
      return
    }
    isCompletingRef.current = true
    isStoppedRef.current = true
    stopPolling()
    stopDeadline()
    setLoading(true)
    try {
      const outcome = await loginWithWechat()
      if (outcome.kind !== 'authenticated') {
        throw new Error('binding did not create a normal session')
      }
      await Taro.reLaunch({ url: '/pages/account/index' })
    } catch {
      setWebUrl(null)
      setError(t('bindingFailed'))
      setLoading(false)
    }
  }, [stopDeadline, stopPolling])

  const pollBinding = useCallback(async () => {
    const bindingId = bindingIdRef.current
    if (
      !foregroundRef.current ||
      bindingId === null ||
      isExpiredRef.current ||
      isCompletingRef.current ||
      isPollInFlightRef.current ||
      isStoppedRef.current
    ) {
      return
    }
    isPollInFlightRef.current = true
    if (Date.now() >= deadlineRef.current) {
      expireBinding()
      isPollInFlightRef.current = false
      return
    }
    try {
      const status = await getBindingStatus(bindingId)
      if (Date.now() >= deadlineRef.current) {
        expireBinding()
        return
      }
      if (!foregroundRef.current || isStoppedRef.current) {
        return
      }
      if (status === 'bound') {
        await completeBinding()
        return
      }
      if (status === 'expired') {
        expireBinding()
      }
    } catch {
      isStoppedRef.current = true
      stopPolling()
      stopDeadline()
      setWebUrl(null)
      setLoading(false)
      setError(t('bindingFailed'))
    } finally {
      isPollInFlightRef.current = false
    }
  }, [completeBinding, expireBinding, stopDeadline, stopPolling])

  const startPolling = useCallback(() => {
    if (!foregroundRef.current || pollTimerRef.current !== null || isExpiredRef.current || isStoppedRef.current) {
      return
    }
    void pollBinding()
    pollTimerRef.current = setInterval(() => {
      void pollBinding()
    }, bindingPollIntervalMs)
  }, [pollBinding])

  const startBinding = useCallback(async () => {
    if (hasStartedRef.current || isExpiredRef.current) {
      return
    }
    hasStartedRef.current = true
    setLoading(true)
    setError(null)
    try {
      const binding = await createBinding()
      bindingIdRef.current = binding.bindingId
      deadlineRef.current = Date.now() + bindingTimeoutMs
      deadlineTimerRef.current = setTimeout(expireBinding, bindingTimeoutMs)
      setWebUrl(binding.webUrl)
      setLoading(false)
      startPolling()
    } catch {
      isStoppedRef.current = true
      stopPolling()
      stopDeadline()
      setLoading(false)
      setError(t('bindingFailed'))
    }
  }, [expireBinding, startPolling, stopDeadline, stopPolling])

  useDidShow(() => {
    foregroundRef.current = true
    if (hasStartedRef.current) {
      startPolling()
      return
    }
    void startBinding()
  })

  useDidHide(() => {
    foregroundRef.current = false
    stopPolling()
  })

  useUnload(() => {
    foregroundRef.current = false
    isStoppedRef.current = true
    stopPolling()
    stopDeadline()
  })

  const handleWebViewError = () => {
    isStoppedRef.current = true
    stopPolling()
    stopDeadline()
    setWebUrl(null)
    setLoading(false)
    setError(t('bindingWebViewFailed'))
  }

  if (webUrl !== null) {
    return <WebView src={webUrl} onError={handleWebViewError} />
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
