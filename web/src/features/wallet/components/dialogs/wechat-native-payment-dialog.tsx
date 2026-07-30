/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { QRCodeSVG } from 'qrcode.react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'

import { getTopupStatus, isApiSuccess } from '../../api'
import type { WechatNativePaymentResponse } from '../../types'

interface WechatNativePaymentDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  payment: WechatNativePaymentResponse['data'] | null
  onPaymentSuccess: () => void
}

export function WechatNativePaymentDialog({
  open,
  onOpenChange,
  payment,
  onPaymentSuccess,
}: WechatNativePaymentDialogProps) {
  const { t } = useTranslation()
  const [status, setStatus] = useState<'pending' | 'success' | 'failed'>(
    'pending'
  )

  useEffect(() => {
    if (!open || !payment) {
      return
    }
    setStatus('pending')
    let disposed = false
    const checkStatus = async () => {
      if (disposed || Date.now() >= payment.expires_at * 1000) {
        if (!disposed) {
          setStatus('failed')
        }
        return
      }
      try {
        const response = await getTopupStatus(payment.trade_no)
        if (disposed || !isApiSuccess(response) || !response.data) {
          return
        }
        if (response.data.status === 'success') {
          setStatus('success')
          onPaymentSuccess()
          return
        }
        if (
          response.data.status === 'failed' ||
          response.data.status === 'expired'
        ) {
          setStatus('failed')
        }
      } catch {
        // Keep the QR code available while a transient status request fails.
      }
    }
    void checkStatus()
    const timer = window.setInterval(() => {
      void checkStatus()
    }, 3000)
    return () => {
      disposed = true
      window.clearInterval(timer)
    }
  }, [onPaymentSuccess, open, payment])

  if (!payment) {
    return null
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('WeChat Pay')}
      description={t('Scan the QR code with WeChat to complete your payment.')}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-[425px]'
      contentHeight='auto'
      bodyClassName='space-y-4 text-center'
      footer={
        <Button variant='outline' onClick={() => onOpenChange(false)}>
          {t('Close')}
        </Button>
      }
    >
      <div className='flex flex-col items-center gap-4 py-3'>
        <QRCodeSVG value={payment.code_url} size={220} includeMargin />
        {status === 'pending' && (
          <p className='text-muted-foreground text-sm'>
            {t('Waiting for payment confirmation...')}
          </p>
        )}
        {status === 'success' && (
          <p className='text-sm font-medium text-green-600'>
            {t('Payment successful')}
          </p>
        )}
        {status === 'failed' && (
          <p className='text-destructive text-sm'>
            {t('Payment expired or was closed. Please create a new order.')}
          </p>
        )}
      </div>
    </Dialog>
  )
}
