import { Button, Input, Picker, Text, View } from '@tarojs/components'
import Taro, { useDidHide } from '@tarojs/taro'
import { useCallback, useEffect, useState } from 'react'

import {
  createMiniAppToken,
  getMiniAppTokens,
  revokeMiniAppToken,
  updateMiniAppTokenStatus,
  type MiniAppTokenSummary,
} from '../../features/tokens/token-service'
import { t } from '../../i18n/strings'
import { MiniAppApiError } from '../../lib/api'
import './index.scss'

const expiryOptions = [7, 30, 90] as const

function shouldReturnToLogin(error: unknown): boolean {
  return error instanceof MiniAppApiError && (
    error.status === 401 ||
    error.code === 'MINIAPP_SESSION_INVALID' ||
    error.code === 'MINIAPP_SESSION_UNAVAILABLE' ||
    error.code === 'AUTH_UNAUTHORIZED'
  )
}

function formatTime(value: number): string {
  if (value <= 0) {
    return '-'
  }
  return new Date(value * 1000).toLocaleString('zh-CN')
}

export default function TokensPage() {
  const [createdTokenKey, setCreatedTokenKey] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [expiresInDays, setExpiresInDays] = useState<7 | 30 | 90>(30)
  const [group, setGroup] = useState('')
  const [loading, setLoading] = useState(true)
  const [models, setModels] = useState('')
  const [name, setName] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [tokens, setTokens] = useState<MiniAppTokenSummary[]>([])

  const loadTokens = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setTokens(await getMiniAppTokens())
    } catch (loadError) {
      if (shouldReturnToLogin(loadError)) {
        setTokens([])
        await Taro.reLaunch({ url: '/pages/index/index' })
        return
      }
      setError(t('tokenLoadFailed'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadTokens()
  }, [loadTokens])

  useDidHide(() => {
    setCreatedTokenKey(null)
  })

  const createToken = async () => {
    const requestedModels = models.split(',').map((model) => model.trim()).filter(Boolean)
    if (name.trim() === '' || group.trim() === '' || requestedModels.length === 0) {
      setError(t('tokenCreateFailed'))
      return
    }
    setSubmitting(true)
    setError(null)
    setCreatedTokenKey(null)
    try {
      const created = await createMiniAppToken({
        name,
        group,
        models: requestedModels,
        expiresInDays,
      })
      setTokens((currentTokens) => [created.token, ...currentTokens])
      setCreatedTokenKey(created.tokenKey)
      setName('')
      setModels('')
    } catch (createError) {
      if (shouldReturnToLogin(createError)) {
        await Taro.reLaunch({ url: '/pages/index/index' })
        return
      }
      setError(t('tokenCreateFailed'))
    } finally {
      setSubmitting(false)
    }
  }

  const changeTokenStatus = async (token: MiniAppTokenSummary) => {
    const status = token.status === 1 ? 2 : 1
    setError(null)
    try {
      const updated = await updateMiniAppTokenStatus(token.id, status)
      setTokens((currentTokens) => currentTokens.map((current) => current.id === updated.id ? updated : current))
    } catch (statusError) {
      if (shouldReturnToLogin(statusError)) {
        await Taro.reLaunch({ url: '/pages/index/index' })
        return
      }
      setError(t('tokenStatusFailed'))
    }
  }

  const revokeToken = async (token: MiniAppTokenSummary) => {
    const confirmation = await Taro.showModal({
      title: t('tokenRevokeTitle'),
      content: t('tokenRevokeConfirm'),
      confirmText: t('tokenRevoke'),
    })
    if (!confirmation.confirm) {
      return
    }
    setError(null)
    try {
      await revokeMiniAppToken(token.id)
      setTokens((currentTokens) => currentTokens.filter((current) => current.id !== token.id))
    } catch (revokeError) {
      if (shouldReturnToLogin(revokeError)) {
        await Taro.reLaunch({ url: '/pages/index/index' })
        return
      }
      setError(t('tokenRevokeFailed'))
    }
  }

  const copyCreatedKey = async () => {
    if (createdTokenKey === null) {
      return
    }
    try {
      await Taro.setClipboardData({ data: createdTokenKey })
      await Taro.showToast({ title: t('tokenCopySuccess'), icon: 'none' })
    } catch {
      setError(t('tokenCopyFailed'))
    }
  }

  return (
    <View className="tokens-shell">
      <Text className="tokens-title">{t('tokens')}</Text>
      {error !== null && <Text className="tokens-error">{error}</Text>}

      {createdTokenKey !== null && (
        <View className="tokens-key-card">
          <Text className="tokens-key-warning">{t('tokenCreatedKey')}</Text>
          <Text className="tokens-key-label">{t('tokenKeyLabel')}</Text>
          <Text className="tokens-key-value" selectable>{createdTokenKey}</Text>
          <View className="tokens-key-actions">
            <Button className="tokens-secondary-button" onClick={() => void copyCreatedKey()}>{t('tokenCopy')}</Button>
            <Button className="tokens-primary-button" onClick={() => setCreatedTokenKey(null)}>{t('tokenCreatedKeySaved')}</Button>
          </View>
        </View>
      )}

      <View className="tokens-form">
        <Text className="tokens-field-label">{t('tokenName')}</Text>
        <Input value={name} maxlength={50} placeholder={t('tokenName')} onInput={(event) => setName(event.detail.value)} />
        <Text className="tokens-field-label">{t('tokenGroup')}</Text>
        <Input value={group} placeholder={t('tokenGroup')} onInput={(event) => setGroup(event.detail.value)} />
        <Text className="tokens-field-label">{t('tokenModels')}</Text>
        <Input value={models} placeholder={t('tokenModels')} onInput={(event) => setModels(event.detail.value)} />
        <Text className="tokens-field-label">{t('tokenExpiresInDays')}</Text>
        <Picker
          mode="selector"
          range={expiryOptions.map(String)}
          value={expiryOptions.indexOf(expiresInDays)}
          onChange={(event) => setExpiresInDays(expiryOptions[Number(event.detail.value)] ?? 30)}
        >
          <View className="tokens-picker">{expiresInDays}</View>
        </Picker>
        <Button className="tokens-primary-button" disabled={submitting} loading={submitting} onClick={() => void createToken()}>
          {t('tokenCreate')}
        </Button>
      </View>

      {loading && <Text className="tokens-loading">{t('refresh')}</Text>}
      {!loading && tokens.length === 0 && <Text className="tokens-empty">{t('tokenEmptyState')}</Text>}
      <View className="tokens-list">
        {tokens.map((token) => (
          <View key={token.id} className="tokens-item">
            <View className="tokens-item-header">
              <Text className="tokens-item-name">{token.name}</Text>
              <Text className="tokens-item-status">{token.status === 1 ? t('tokenToggleDisable') : t('tokenToggleEnable')}</Text>
            </View>
            <Text className="tokens-item-key">{token.keyHint}</Text>
            <Text className="tokens-item-detail">{token.group}</Text>
            <Text className="tokens-item-detail">{token.modelLimits.join(', ')}</Text>
            <Text className="tokens-item-detail">{formatTime(token.expiresAt)}</Text>
            <View className="tokens-item-actions">
              <Button className="tokens-secondary-button" onClick={() => void changeTokenStatus(token)}>
                {token.status === 1 ? t('tokenToggleDisable') : t('tokenToggleEnable')}
              </Button>
              <Button className="tokens-danger-button" onClick={() => void revokeToken(token)}>{t('tokenRevoke')}</Button>
            </View>
          </View>
        ))}
      </View>
    </View>
  )
}
