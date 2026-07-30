/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQueryClient } from '@tanstack/react-query'
import * as React from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import {
  getDirectPaymentConfigStatus,
  saveAlipayDirectConfig,
  saveWechatPayDirectConfig,
} from './direct-payment-api'

export type DirectPaymentSettingsValues = {
  alipayEnabled: boolean
  alipayAppID: string
  alipaySellerID: string
  wechatEnabled: boolean
  wechatAppID: string
  wechatMerchantID: string
  wechatMerchantCertificateSerial: string
}

interface DirectPaymentSettingsSectionProps {
  defaults: DirectPaymentSettingsValues
  provider: 'alipay' | 'wechat'
}

export function DirectPaymentSettingsSection({
  defaults,
  provider,
}: DirectPaymentSettingsSectionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [alipayEnabled, setAlipayEnabled] = React.useState(
    defaults.alipayEnabled
  )
  const [alipayAppID, setAlipayAppID] = React.useState(defaults.alipayAppID)
  const [alipaySellerID, setAlipaySellerID] = React.useState(
    defaults.alipaySellerID
  )
  const [alipayAppPrivateKey, setAlipayAppPrivateKey] = React.useState('')
  const [alipayPublicKey, setAlipayPublicKey] = React.useState('')
  const [wechatEnabled, setWechatEnabled] = React.useState(
    defaults.wechatEnabled
  )
  const [wechatAppID, setWechatAppID] = React.useState(defaults.wechatAppID)
  const [wechatMerchantID, setWechatMerchantID] = React.useState(
    defaults.wechatMerchantID
  )
  const [wechatSerial, setWechatSerial] = React.useState(
    defaults.wechatMerchantCertificateSerial
  )
  const [wechatPrivateKey, setWechatPrivateKey] = React.useState('')
  const [wechatAPIv3Key, setWechatAPIv3Key] = React.useState('')
  const [saving, setSaving] = React.useState<'alipay' | 'wechat' | null>(null)
  const [configured, setConfigured] = React.useState({
    alipay: false,
    wechat: false,
  })

  React.useEffect(() => {
    setAlipayEnabled(defaults.alipayEnabled)
    setAlipayAppID(defaults.alipayAppID)
    setAlipaySellerID(defaults.alipaySellerID)
    setWechatEnabled(defaults.wechatEnabled)
    setWechatAppID(defaults.wechatAppID)
    setWechatMerchantID(defaults.wechatMerchantID)
    setWechatSerial(defaults.wechatMerchantCertificateSerial)
  }, [defaults])

  React.useEffect(() => {
    void getDirectPaymentConfigStatus().then((response) => {
      if (response.message === 'success' && response.data) {
        setConfigured({
          alipay: response.data.alipay.configured,
          wechat: response.data.wechat.configured,
        })
      }
    })
  }, [])

  const saveAlipay = async () => {
    try {
      setSaving('alipay')
      const response = await saveAlipayDirectConfig({
        enabled: alipayEnabled,
        appID: alipayAppID.trim(),
        sellerID: alipaySellerID.trim(),
        appPrivateKey: alipayAppPrivateKey.trim(),
        alipayPublicKey: alipayPublicKey.trim(),
      })
      if (response.message !== 'success') {
        toast.error(t('Failed to save payment settings'))
        return
      }
      setAlipayAppPrivateKey('')
      setAlipayPublicKey('')
      setConfigured((current) => ({
        ...current,
        alipay: Boolean(response.data?.configured),
      }))
      await queryClient.invalidateQueries({ queryKey: ['system-options'] })
      toast.success(t('Payment settings saved'))
    } catch {
      toast.error(t('Failed to save payment settings'))
    } finally {
      setSaving(null)
    }
  }

  const saveWechat = async () => {
    try {
      setSaving('wechat')
      const response = await saveWechatPayDirectConfig({
        enabled: wechatEnabled,
        appID: wechatAppID.trim(),
        merchantID: wechatMerchantID.trim(),
        merchantCertificateSerial: wechatSerial.trim(),
        merchantPrivateKey: wechatPrivateKey.trim(),
        apiV3Key: wechatAPIv3Key.trim(),
      })
      if (response.message !== 'success') {
        toast.error(t('Failed to save payment settings'))
        return
      }
      setWechatPrivateKey('')
      setWechatAPIv3Key('')
      setConfigured((current) => ({
        ...current,
        wechat: Boolean(response.data?.configured),
      }))
      await queryClient.invalidateQueries({ queryKey: ['system-options'] })
      toast.success(t('Payment settings saved'))
    } catch {
      toast.error(t('Failed to save payment settings'))
    } finally {
      setSaving(null)
    }
  }

  return (
    <div className='space-y-8'>
      <Alert>
        <AlertTitle>{t('Official payment callback URLs')}</AlertTitle>
        <AlertDescription>
          <p>
            {t('Official payment requires a public HTTPS callback address.')}
          </p>
          <code className='mt-2 block'>/api/alipay/notify</code>
          <code className='block'>/api/wechat/notify</code>
        </AlertDescription>
      </Alert>

      {provider === 'alipay' && (
        <section className='space-y-4'>
          <div className='flex items-center justify-between gap-4'>
            <div>
              <h3 className='font-medium'>{t('Official Alipay')}</h3>
              <p className='text-muted-foreground text-sm'>
                {configured.alipay
                  ? t('Configuration complete')
                  : t('Configuration incomplete')}
              </p>
            </div>
            <Switch
              checked={alipayEnabled}
              onCheckedChange={setAlipayEnabled}
            />
          </div>
          <div className='grid gap-4 md:grid-cols-2'>
            <Input
              value={alipayAppID}
              onChange={(event) => setAlipayAppID(event.target.value)}
              placeholder={t('Alipay App ID')}
            />
            <Input
              value={alipaySellerID}
              onChange={(event) => setAlipaySellerID(event.target.value)}
              placeholder={t('Alipay seller ID')}
            />
            <Input
              type='password'
              value={alipayAppPrivateKey}
              onChange={(event) => setAlipayAppPrivateKey(event.target.value)}
              placeholder={t('Application private key')}
              autoComplete='new-password'
            />
            <Input
              type='password'
              value={alipayPublicKey}
              onChange={(event) => setAlipayPublicKey(event.target.value)}
              placeholder={t('Alipay public key')}
              autoComplete='new-password'
            />
          </div>
          <p className='text-muted-foreground text-sm'>
            {t('Leave blank to keep the saved secret')}
          </p>
          <Button type='button' onClick={saveAlipay} disabled={saving !== null}>
            {t('Save Alipay configuration')}
          </Button>
        </section>
      )}

      {provider === 'wechat' && (
        <section className='space-y-4 border-t pt-6'>
          <div className='flex items-center justify-between gap-4'>
            <div>
              <h3 className='font-medium'>{t('Official WeChat Pay')}</h3>
              <p className='text-muted-foreground text-sm'>
                {configured.wechat
                  ? t('Configuration complete')
                  : t('Configuration incomplete')}
              </p>
            </div>
            <Switch
              checked={wechatEnabled}
              onCheckedChange={setWechatEnabled}
            />
          </div>
          <div className='grid gap-4 md:grid-cols-2'>
            <Input
              value={wechatAppID}
              onChange={(event) => setWechatAppID(event.target.value)}
              placeholder={t('WeChat App ID')}
            />
            <Input
              value={wechatMerchantID}
              onChange={(event) => setWechatMerchantID(event.target.value)}
              placeholder={t('WeChat merchant ID')}
            />
            <Input
              value={wechatSerial}
              onChange={(event) => setWechatSerial(event.target.value)}
              placeholder={t('Merchant certificate serial number')}
            />
            <Input
              type='password'
              value={wechatPrivateKey}
              onChange={(event) => setWechatPrivateKey(event.target.value)}
              placeholder={t('Merchant private key')}
              autoComplete='new-password'
            />
            <Input
              type='password'
              value={wechatAPIv3Key}
              onChange={(event) => setWechatAPIv3Key(event.target.value)}
              placeholder={t('APIv3 key')}
              autoComplete='new-password'
            />
          </div>
          <p className='text-muted-foreground text-sm'>
            {t('Leave blank to keep the saved secret')}
          </p>
          <Button type='button' onClick={saveWechat} disabled={saving !== null}>
            {t('Save WeChat Pay configuration')}
          </Button>
        </section>
      )}
    </div>
  )
}
