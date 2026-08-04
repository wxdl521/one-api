import { Button, Picker, Text, Textarea, View } from '@tarojs/components'
import Taro, { useDidHide, useDidShow, useUnload } from '@tarojs/taro'
import { useCallback, useEffect, useRef, useState } from 'react'

import {
  createMiniTextTestRequestID,
  getMiniTextTestModels,
  getMiniTextTestStatus,
  isMiniTextTestInputValid,
  miniTextTestMaxInputCodePoints,
  startMiniTextTest,
  type MiniTextTestStatus,
} from '../../features/text-test/text-test-service'
import { TextTestLifecycle } from '../../features/text-test/text-test-lifecycle'
import { t } from '../../i18n/strings'
import { MiniAppApiError } from '../../lib/api'
import { getPendingTextTestRequestID } from '../../lib/pending-text-test'
import './index.scss'

function shouldReturnToLogin(error: unknown): boolean {
  return error instanceof MiniAppApiError && (
    error.status === 401 ||
    error.code === 'MINIAPP_SESSION_INVALID' ||
    error.code === 'MINIAPP_SESSION_UNAVAILABLE' ||
    error.code === 'AUTH_UNAUTHORIZED'
  )
}

function statusMessage(status: MiniTextTestStatus): string {
  if (status.state === 'succeeded') {
    return t('textTestSucceeded')
  }
  if (status.state === 'timed_out') {
    return t('textTestTimedOut')
  }
  if (status.state === 'failed') {
    return t('textTestFailed')
  }
  return t('textTestRunning')
}

export default function TextTestPage() {
  const [error, setError] = useState<string | null>(null)
  const [input, setInput] = useState('')
  const [loadingModels, setLoadingModels] = useState(true)
  const [models, setModels] = useState<string[]>([])
  const [pendingRequestID, setPendingRequestID] = useState<string | null>(getPendingTextTestRequestID)
  const [selectedModel, setSelectedModel] = useState('')
  const [starting, setStarting] = useState(false)
  const [status, setStatus] = useState<MiniTextTestStatus | null>(null)
  const isMountedRef = useRef(true)
  const lifecycleRef = useRef<TextTestLifecycle | null>(null)

  if (lifecycleRef.current === null) {
    lifecycleRef.current = new TextTestLifecycle({
      createRequestID: createMiniTextTestRequestID,
      getStatus: getMiniTextTestStatus,
      isRetryableError: (requestError) => !(requestError instanceof MiniAppApiError && (
        requestError.code === 'MINIAPP_TEXT_TEST_DISABLED' ||
        requestError.code === 'MINIAPP_TEXT_TEST_INVALID' ||
        requestError.code === 'MINIAPP_TEXT_TEST_MODEL_UNAVAILABLE' ||
        requestError.code === 'MINIAPP_TEXT_TEST_REQUEST_CONFLICT' ||
        shouldReturnToLogin(requestError)
      )),
      onError: (requestError) => {
        if (shouldReturnToLogin(requestError)) {
          lifecycleRef.current?.resetSession()
          void Taro.reLaunch({ url: '/pages/index/index' })
          return
        }
        if (isMountedRef.current) {
          setError(t('textTestUnavailable'))
        }
      },
      onPending: (_requestID, key) => {
        if (isMountedRef.current) {
          setError(t(key))
        }
      },
      onRequestIDChange: (requestID) => {
        if (isMountedRef.current) {
          setPendingRequestID(requestID)
        }
      },
      onStartChange: setStarting,
      onStatus: (nextStatus) => {
        if (isMountedRef.current) {
          setError(null)
          setStatus(nextStatus)
        }
      },
      start: startMiniTextTest,
    })
  }

  const loadModels = useCallback(async () => {
    setLoadingModels(true)
    setError(null)
    try {
      const availableModels = await getMiniTextTestModels()
      if (!isMountedRef.current) {
        return
      }
      setModels(availableModels)
      setSelectedModel((currentModel) => availableModels.includes(currentModel) ? currentModel : (availableModels[0] ?? ''))
    } catch (loadError) {
      if (!isMountedRef.current) {
        return
      }
      if (shouldReturnToLogin(loadError)) {
        lifecycleRef.current?.resetSession()
        await Taro.reLaunch({ url: '/pages/index/index' })
        return
      }
      setModels([])
      setSelectedModel('')
      setError(t('textTestUnavailable'))
    } finally {
      if (isMountedRef.current) {
        setLoadingModels(false)
      }
    }
  }, [])

  useEffect(() => {
    void loadModels()
    return () => {
      isMountedRef.current = false
      lifecycleRef.current?.unload()
    }
  }, [loadModels])

  useDidShow(() => {
    lifecycleRef.current?.show()
  })

  useDidHide(() => {
    lifecycleRef.current?.hide()
  })

  useUnload(() => {
    isMountedRef.current = false
    lifecycleRef.current?.unload()
  })

  const submit = async () => {
    if (pendingRequestID !== null) {
      setError(null)
      await lifecycleRef.current?.submit({ model: selectedModel, input })
      return
    }
    if (!isMiniTextTestInputValid(input)) {
      setError(t('textTestInputLimit'))
      return
    }
    if (selectedModel === '') {
      setError(t('textTestNoModels'))
      return
    }
    setError(null)
    await lifecycleRef.current?.submit({ model: selectedModel, input })
  }

  const selectedModelIndex = Math.max(models.indexOf(selectedModel), 0)
  const inputIsValid = isMiniTextTestInputValid(input)
  const actionDisabled = starting || (pendingRequestID === null && (loadingModels || selectedModel === '' || !inputIsValid))

  return (
    <View className="text-test-shell">
      <Text className="text-test-title">{t('textTest')}</Text>
      <Text className="text-test-description">{t('textTestNoAttachments')}</Text>
      <View className="text-test-privacy-card">
        <Text>{t('textTestPrivacy')}</Text>
      </View>

      {loadingModels && <Text className="text-test-loading">{t('textTestLoading')}</Text>}
      {!loadingModels && models.length === 0 && <Text className="text-test-empty">{t('textTestNoModels')}</Text>}

      <View className="text-test-form">
        <Text className="text-test-label">{t('textTestModel')}</Text>
        <Picker
          disabled={loadingModels || models.length === 0 || pendingRequestID !== null}
          mode="selector"
          range={models}
          value={selectedModelIndex}
          onChange={(event) => setSelectedModel(models[Number(event.detail.value)] ?? '')}
        >
          <View className="text-test-picker">{selectedModel || t('textTestNoModels')}</View>
        </Picker>

        <Text className="text-test-label">{t('textTestInput')}</Text>
        <Textarea
          autoHeight
          className="text-test-input"
          disabled={pendingRequestID !== null}
          maxlength={miniTextTestMaxInputCodePoints}
          placeholder={t('textTestInput')}
          value={input}
          onInput={(event) => setInput(event.detail.value)}
        />
        <Text className="text-test-count">{Array.from(input).length}/{miniTextTestMaxInputCodePoints}</Text>

        <Button
          className="text-test-submit"
          disabled={actionDisabled}
          loading={starting || (pendingRequestID !== null && status?.state === 'running')}
          onClick={() => void submit()}
        >
          {pendingRequestID === null ? t('textTestSubmit') : t('textTestCheckStatus')}
        </Button>
      </View>

      {error !== null && <Text className="text-test-error">{error}</Text>}
      {status !== null && (
        <View className="text-test-status-card">
          <Text className="text-test-status-message">{statusMessage(status)}</Text>
          <Text>{t('textTestRequest')}: {status.requestID}</Text>
          {status.state !== 'running' && <Text>{t('textTestCharge')}: {status.chargedQuota}</Text>}
        </View>
      )}
    </View>
  )
}
