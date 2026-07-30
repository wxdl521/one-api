/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { api } from '@/lib/api'

type DirectPaymentConfigStatus = {
  alipay: { enabled: boolean; configured: boolean }
  wechat: { enabled: boolean; configured: boolean }
}

type ApiResponse<T> = { message?: string; data?: T }

export async function getDirectPaymentConfigStatus() {
  const response = await api.get<ApiResponse<DirectPaymentConfigStatus>>(
    '/api/option/direct-payment/status'
  )
  return response.data
}

export async function saveAlipayDirectConfig(params: {
  enabled: boolean
  appID: string
  sellerID: string
  appPrivateKey: string
  alipayPublicKey: string
}) {
  const response = await api.post<ApiResponse<{ configured: boolean }>>(
    '/api/option/alipay/save',
    {
      enabled: params.enabled,
      app_id: params.appID,
      seller_id: params.sellerID,
      app_private_key: params.appPrivateKey,
      alipay_public_key: params.alipayPublicKey,
    }
  )
  return response.data
}

export async function saveWechatPayDirectConfig(params: {
  enabled: boolean
  appID: string
  merchantID: string
  merchantCertificateSerial: string
  merchantPrivateKey: string
  apiV3Key: string
}) {
  const response = await api.post<ApiResponse<{ configured: boolean }>>(
    '/api/option/wechat-pay/save',
    {
      enabled: params.enabled,
      app_id: params.appID,
      merchant_id: params.merchantID,
      merchant_certificate_serial: params.merchantCertificateSerial,
      merchant_private_key: params.merchantPrivateKey,
      api_v3_key: params.apiV3Key,
    }
  )
  return response.data
}
