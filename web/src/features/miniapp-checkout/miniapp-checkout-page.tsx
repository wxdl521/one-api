/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useMutation } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import {
  consumeMiniAppCheckoutBootstrapTicket,
  consumeMiniAppCheckoutURL,
  createMiniAppCheckoutConfirmationPayload,
} from './lib/checkout-ticket'

let browserCheckoutTicket: string | null | undefined

function captureCheckoutTicketFromLocation() {
  if (typeof window === 'undefined') return null
  const captured = consumeMiniAppCheckoutURL(window.location.href)
  if (captured === null) return null
  window.history.replaceState(window.history.state, '', captured.visibleURL)
  return captured.checkoutTicket
}

function consumeBootstrapCheckoutTicket() {
  if (typeof window === 'undefined') return null
  return consumeMiniAppCheckoutBootstrapTicket(
    window as unknown as Record<string, unknown>
  )
}

// This runs during module evaluation, before React renders this route or any
// route-level instrumentation can observe the handoff URL. The synchronous
// head bootstrap has already removed the fragment and kept the ticket in
// memory only for the confirmation request.
if (
  typeof window !== 'undefined' &&
  window.location.pathname === '/miniapp-checkout'
) {
  browserCheckoutTicket =
    consumeBootstrapCheckoutTicket() ?? captureCheckoutTicketFromLocation()
}

function checkoutTicketFromMemory() {
  if (browserCheckoutTicket !== undefined) return browserCheckoutTicket
  browserCheckoutTicket =
    consumeBootstrapCheckoutTicket() ?? captureCheckoutTicketFromLocation()
  return browserCheckoutTicket
}

function isAllowedCheckoutPath(value: unknown): value is string {
  if (value === '/subscriptions') return true
  return typeof value === 'string' && /^\/products\/[1-9]\d*$/.test(value)
}

export function MiniAppCheckoutPage() {
  const { t } = useTranslation()
  const auth = useAuthStore((state) => state.auth)
  const [checkoutTicket] = useState(checkoutTicketFromMemory)
  const confirmationAttempted = useRef(false)
  const {
    data: checkoutPath,
    isError,
    isPending,
    isSuccess,
    mutate: confirmCheckout,
  } = useMutation({
    mutationFn: async () => {
      const payload = checkoutTicket
        ? createMiniAppCheckoutConfirmationPayload(checkoutTicket)
        : null
      if (payload === null) {
        throw new Error('invalid mini app checkout ticket')
      }
      const response = await api.post(
        '/api/miniapp/checkout/confirm',
        payload,
        {
          skipErrorHandler: true,
        }
      )
      const checkoutPath = response.data?.data?.checkout_path
      if (
        response.data?.success !== true ||
        !isAllowedCheckoutPath(checkoutPath)
      ) {
        throw new Error('mini app checkout confirmation failed')
      }
      browserCheckoutTicket = null
      return checkoutPath
    },
  })

  useEffect(() => {
    if (
      checkoutTicket !== null &&
      auth.user !== null &&
      auth.accessToken !== null &&
      !confirmationAttempted.current &&
      !isPending &&
      !isSuccess
    ) {
      confirmationAttempted.current = true
      confirmCheckout()
    }
  }, [
    auth.accessToken,
    auth.user,
    checkoutTicket,
    confirmCheckout,
    isPending,
    isSuccess,
  ])

  const needsSignIn = auth.user === null || auth.accessToken === null

  return (
    <main className='bg-muted/30 flex min-h-screen items-center justify-center p-4'>
      <Card className='w-full max-w-md'>
        <CardHeader>
          <CardTitle>{t('Continue checkout')}</CardTitle>
          <CardDescription>
            {t('Sign in to continue with the existing checkout flow.')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {checkoutTicket === null && (
            <Alert variant='destructive'>
              <AlertTitle>{t('Checkout link unavailable')}</AlertTitle>
              <AlertDescription>
                {t(
                  'This Mini Program checkout link is invalid or has expired.'
                )}
              </AlertDescription>
            </Alert>
          )}
          {isError && (
            <Alert variant='destructive'>
              <AlertTitle>{t('Checkout could not be confirmed')}</AlertTitle>
              <AlertDescription>
                {t('Return to the Mini Program and start checkout again.')}
              </AlertDescription>
            </Alert>
          )}
          {checkoutPath !== undefined && (
            <Alert>
              <AlertTitle>{t('Checkout ready')}</AlertTitle>
              <AlertDescription>
                {t('Continue in the existing Web checkout flow.')}
              </AlertDescription>
            </Alert>
          )}
        </CardContent>
        {checkoutTicket !== null && needsSignIn && (
          <CardFooter className='justify-end'>
            <Button
              render={
                <Link
                  to='/sign-in'
                  search={{ redirect: '/miniapp-checkout' }}
                />
              }
            >
              {t('Sign in')}
            </Button>
          </CardFooter>
        )}
        {checkoutPath !== undefined && (
          <CardFooter className='justify-end'>
            <Button render={<a href={checkoutPath} />}>
              {t('Continue checkout')}
            </Button>
          </CardFooter>
        )}
      </Card>
    </main>
  )
}
