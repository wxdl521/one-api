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
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
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
import { api, clearAuthentication } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import {
  clearMiniAppBindingSessionTicket,
  consumeMiniAppBindingBootstrapTicket,
  consumeMiniAppBindingURL,
  createMiniAppBindingConfirmationPayload,
  miniAppBindPath,
  readMiniAppBindingSessionTicket,
  rememberMiniAppBindingSessionTicket,
} from './lib/binding-ticket'

let initialBindingTicket: string | null | undefined

function captureBindingTicketFromLocation() {
  if (typeof window === 'undefined') return null
  const captured = consumeMiniAppBindingURL(window.location.href)
  if (captured === null) return null
  window.history.replaceState(window.history.state, '', captured.visibleURL)
  if (captured.bindingTicket !== null) {
    rememberMiniAppBindingSessionTicket(captured.bindingTicket)
  }
  return captured.bindingTicket
}

function consumeBootstrapBindingTicket() {
  if (typeof window === 'undefined') return null
  return consumeMiniAppBindingBootstrapTicket(
    window as unknown as Record<string, unknown>
  )
}

// This runs during module evaluation, before React renders this route or any
// route-level instrumentation can observe the handoff URL. The synchronous
// head bootstrap has already removed the fragment and left the ticket in
// memory and tab-scoped storage for the confirmation request. The latter is
// needed only when browser authentication navigates away and returns here.
if (
  typeof window !== 'undefined' &&
  window.location.pathname === '/miniapp-bind'
) {
  initialBindingTicket =
    consumeBootstrapBindingTicket() ?? captureBindingTicketFromLocation()
}

function takeBindingTicketFromLocation() {
  if (initialBindingTicket !== undefined) {
    const bindingTicket = initialBindingTicket
    initialBindingTicket = undefined
    return bindingTicket
  }
  const ticket =
    consumeBootstrapBindingTicket() ?? captureBindingTicketFromLocation()
  if (ticket !== null) {
    rememberMiniAppBindingSessionTicket(ticket)
    return ticket
  }
  return readMiniAppBindingSessionTicket()
}

export function MiniAppBindPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const hasAccessToken = useAuthStore(
    (state) => state.auth.accessToken !== null
  )
  const [bindingTicket] = useState(takeBindingTicketFromLocation)
  const continueAtSignIn = () => {
    if (bindingTicket !== null) {
      rememberMiniAppBindingSessionTicket(bindingTicket)
    }
    void navigate({
      to: '/sign-in',
      search: { redirect: miniAppBindPath },
      replace: true,
    })
  }
  const mutation = useMutation({
    mutationFn: async () => {
      const payload = bindingTicket
        ? createMiniAppBindingConfirmationPayload(bindingTicket)
        : null
      if (payload === null) throw new Error('invalid mini app binding ticket')
      const response = await api.post(
        '/api/miniapp/bindings/confirm',
        payload,
        {
          skipErrorHandler: true,
          skipAuthRefresh: true,
        }
      )
      if (response.data?.success !== true) {
        throw new Error('mini app binding confirmation failed')
      }
    },
    onSuccess: () => {
      clearMiniAppBindingSessionTicket()
    },
    onError: (error: unknown) => {
      const status = (error as { response?: { status?: unknown } })?.response
        ?.status
      if (status === 401) {
        clearAuthentication()
        continueAtSignIn()
        return
      }
      if (
        status === 400 ||
        status === 403 ||
        status === 409 ||
        status === 410
      ) {
        clearMiniAppBindingSessionTicket()
      }
    },
  })

  const isInvalid = bindingTicket === null
  const isConfirmed = mutation.isSuccess
  let confirmationLabel = t('Confirm binding')
  if (!hasAccessToken) {
    confirmationLabel = t('Sign in')
  } else if (mutation.isPending) {
    confirmationLabel = t('Confirming binding...')
  }

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
              onClick={() => {
                if (!hasAccessToken) {
                  continueAtSignIn()
                  return
                }
                mutation.mutate()
              }}
              disabled={mutation.isPending}
            >
              {confirmationLabel}
            </Button>
          </CardFooter>
        )}
      </Card>
    </main>
  )
}
