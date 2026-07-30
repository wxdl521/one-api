/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import i18next from 'i18next'
import { useCallback, useState } from 'react'
import { toast } from 'sonner'

import { isApiSuccess, requestWechatNativePayment } from '../api'
import type { WechatNativePaymentResponse } from '../types'

export function useWechatNativePayment() {
  const [processing, setProcessing] = useState(false)

  const processWechatNativePayment = useCallback(async (amount: number) => {
    try {
      setProcessing(true)
      const response: WechatNativePaymentResponse =
        await requestWechatNativePayment({ amount: Math.floor(amount) })
      if (!isApiSuccess(response) || !response.data?.code_url) {
        toast.error(response.message || i18next.t('Payment request failed'))
        return null
      }
      return response.data
    } catch {
      toast.error(i18next.t('Payment request failed'))
      return null
    } finally {
      setProcessing(false)
    }
  }, [])

  return { processing, processWechatNativePayment }
}
