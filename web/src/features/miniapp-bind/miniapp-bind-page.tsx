/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMutation } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
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

import {
  createMiniAppBindingConfirmationPayload,
  miniAppBindURLWithoutTicket,
} from './lib/binding-ticket'

interface MiniAppBindPageProps {
  bindingTicket?: string
}

const bindingTicketStorageKey = 'miniapp:binding-ticket'

function captureBindingTicket(bindingTicket?: string) {
  if (typeof window === 'undefined') return null
  const cleanedURL = miniAppBindURLWithoutTicket(window.location.href)
  const payload = bindingTicket
    ? createMiniAppBindingConfirmationPayload(bindingTicket)
    : null
  if (cleanedURL === null || payload === null) return null
  window.sessionStorage.setItem(bindingTicketStorageKey, payload.binding_ticket)
  return payload.binding_ticket
}

function storedBindingTicket() {
  if (typeof window === 'undefined') return null
  const ticket = window.sessionStorage.getItem(bindingTicketStorageKey)
  if (!ticket) return null
  return createMiniAppBindingConfirmationPayload(ticket)?.binding_ticket ?? null
}

export function MiniAppBindPage(props: MiniAppBindPageProps) {
  const { t } = useTranslation()
  const [bindingTicket] = useState(() => {
    const capturedTicket = captureBindingTicket(props.bindingTicket)
    if (capturedTicket !== null) return capturedTicket
    if (typeof window !== 'undefined' && window.location.search !== '') {
      return null
    }
    return storedBindingTicket()
  })
  const mutation = useMutation({
    mutationFn: async () => {
      const payload = bindingTicket
        ? createMiniAppBindingConfirmationPayload(bindingTicket)
        : null
      if (payload === null) throw new Error('invalid mini app binding ticket')
      const response = await api.post('/api/miniapp/bindings/confirm', payload, {
        skipErrorHandler: true,
      })
      if (response.data?.success !== true) {
        throw new Error('mini app binding confirmation failed')
      }
    },
    onSuccess: () => {
      window.sessionStorage.removeItem(bindingTicketStorageKey)
    },
  })

  useEffect(() => {
    const cleanedURL = miniAppBindURLWithoutTicket(window.location.href)
    const fallbackURL = `${window.location.pathname}${window.location.hash}`
    window.history.replaceState(
      window.history.state,
      '',
      cleanedURL ?? fallbackURL
    )
  }, [])

  const isInvalid = bindingTicket === null
  const isConfirmed = mutation.isSuccess

  return (
    <main className='bg-muted/30 flex min-h-screen items-center justify-center p-4'>
      <Card className='w-full max-w-md'>
        <CardHeader>
          <CardTitle>{t('Confirm Mini Program binding')}</CardTitle>
          <CardDescription>
            {t(
              'Confirm that you want to link this browser account to the Mini Program.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {isInvalid && (
            <Alert variant='destructive'>
              <AlertTitle>{t('Binding link unavailable')}</AlertTitle>
              <AlertDescription>
                {t('This Mini Program binding link is invalid or has expired.')}
              </AlertDescription>
            </Alert>
          )}
          {mutation.isError && (
            <Alert variant='destructive'>
              <AlertTitle>{t('Binding could not be confirmed')}</AlertTitle>
              <AlertDescription>
                {t(
                  'We could not complete this binding. Start again from the Mini Program.'
                )}
              </AlertDescription>
            </Alert>
          )}
          {isConfirmed && (
            <Alert>
              <AlertTitle>{t('Binding confirmed')}</AlertTitle>
              <AlertDescription>
                {t(
                  'Mini Program binding confirmed. You can return to the Mini Program.'
                )}
              </AlertDescription>
            </Alert>
          )}
        </CardContent>
        {!isInvalid && !isConfirmed && (
          <CardFooter className='justify-end'>
            <Button
              type='button'
              onClick={() => mutation.mutate()}
              disabled={mutation.isPending}
            >
              {mutation.isPending
                ? t('Confirming binding...')
                : t('Confirm binding')}
            </Button>
          </CardFooter>
        )}
      </Card>
    </main>
  )
}
