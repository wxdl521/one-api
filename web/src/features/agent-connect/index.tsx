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
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { logout } from '@/features/auth/api'
import { clearAuthenticatedClientState } from '@/lib/auth-session'
import { useAuthStore } from '@/stores/auth-store'

import {
  authorizeAgentConnect,
  beginAgentConnectReauthentication,
  cancelAgentConnect,
  getAgentConnectOptions,
  type AgentConnectGroup,
} from './api'

const EMPTY_GROUPS: AgentConnectGroup[] = []
const EMPTY_MODELS: string[] = []
const ONBOARDING_INSTRUCTION =
  'Read https://the-one.bolierxiang.cn/skills/myagents/SKILL.md and safely connect The One. Keep my current default provider unchanged.'

type AgentConnectPageProps = {
  requestID?: string
}

export function AgentConnectPage({ requestID }: AgentConnectPageProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const auth = useAuthStore((state) => state.auth)
  const [group, setGroup] = useState('')
  const queryClient = useQueryClient()
  const reauthenticationRedirected = useRef(false)
  const [model, setModel] = useState('')
  const [connectionApproved, setConnectionApproved] = useState(false)
  const authenticated = Boolean(auth.user && auth.accessToken)
  const reauthenticationQuery = useQuery({
    queryKey: ['agent-connect-reauthentication', requestID],
    queryFn: () => beginAgentConnectReauthentication(requestID ?? ''),
    enabled: Boolean(requestID) && auth.bootstrapState === 'complete',
    retry: false,
  })

  useEffect(() => {
    if (
      reauthenticationRedirected.current ||
      !requestID ||
      !reauthenticationQuery.data?.success ||
      reauthenticationQuery.data.data?.reauthentication_required !== true
    ) {
      return
    }
    reauthenticationRedirected.current = true
    void (async () => {
      try {
        await logout()
      } finally {
        clearAuthenticatedClientState(queryClient, false)
        const redirect = `${window.location.pathname}${window.location.search}`
        void navigate({ to: '/sign-in', search: { redirect }, replace: true })
      }
    })()
  }, [navigate, queryClient, reauthenticationQuery.data, requestID])

  const optionsQuery = useQuery({
    queryKey: ['agent-connect-options', requestID],
    queryFn: () => getAgentConnectOptions(requestID ?? ''),
    enabled:
      authenticated &&
      reauthenticationQuery.data?.success === true &&
      reauthenticationQuery.data.data?.reauthentication_required === false,
    retry: false,
  })
  const groups = useMemo(() => {
    if (!optionsQuery.data?.success) return EMPTY_GROUPS
    return optionsQuery.data.data?.groups ?? EMPTY_GROUPS
  }, [optionsQuery.data])
  const selectedGroup = useMemo(
    () => groups.find((item) => item.id === group),
    [group, groups]
  )
  const models = selectedGroup?.models ?? EMPTY_MODELS

  useEffect(() => {
    if (groups.length === 0) {
      setGroup('')
      return
    }
    if (!groups.some((item) => item.id === group)) {
      setGroup(groups[0].id)
    }
  }, [group, groups])

  useEffect(() => {
    if (models.length === 0) {
      setModel('')
      return
    }
    if (!models.includes(model)) {
      setModel(models[0])
    }
  }, [model, models])

  const authorizeMutation = useMutation({
    mutationFn: () => authorizeAgentConnect(requestID ?? '', { group, model }),
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to complete the connection.'))
        return
      }
      if (response.data.completed) {
        setConnectionApproved(true)
        return
      }
      if (response.data.callback_url) {
        window.location.assign(response.data.callback_url)
        return
      }
      toast.error(response.message || t('Failed to complete the connection.'))
    },
    onError: () => toast.error(t('Failed to complete the connection.')),
  })
  const cancelMutation = useMutation({
    mutationFn: () => cancelAgentConnect(requestID ?? ''),
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to complete the connection.'))
        return
      }
      toast.success(t('Connection canceled.'))
      void navigate({ to: '/' })
    },
    onError: () => toast.error(t('Failed to complete the connection.')),
  })

  const signIn = () => {
    const redirect = `${window.location.pathname}${window.location.search}`
    void navigate({ to: '/sign-in', search: { redirect } })
  }

  let content: ReactNode
  if (connectionApproved) {
    content = (
      <Card>
        <CardHeader>
          <CardTitle>{t('Connection approved')}</CardTitle>
          <CardDescription>
            {t(
              'The connection is approved. Return to Hermes or MyAgents; it will finish local setup without changing your default provider.'
            )}
          </CardDescription>
        </CardHeader>
      </Card>
    )
  } else if (!requestID) {
    content = (
      <Card>
        <CardHeader>
          <CardTitle>{t('Connect from MyAgents')}</CardTitle>
          <CardDescription>
            {t(
              'Send this instruction to MyAgents. It will open this official page and wait while you sign in and approve the connection.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='bg-muted flex items-start gap-2 rounded-lg p-3'>
            <code className='min-w-0 flex-1 overflow-x-auto text-xs leading-5 break-all'>
              {ONBOARDING_INSTRUCTION}
            </code>
          </div>
        </CardContent>
      </Card>
    )
  } else if (
    auth.bootstrapState !== 'complete' ||
    reauthenticationQuery.isLoading ||
    reauthenticationQuery.data?.data?.reauthentication_required === true
  ) {
    content = <LoadingState message={t('Sign in to continue')} />
  } else if (
    reauthenticationQuery.isError ||
    !reauthenticationQuery.data?.success
  ) {
    content = (
      <ErrorState title={t('The connection request could not be loaded.')} />
    )
  } else if (!authenticated) {
    content = (
      <Card>
        <CardHeader>
          <CardTitle>{t('Sign in to continue')}</CardTitle>
          <CardDescription>
            {t(
              'Sign in on this official page, choose a group and model, then confirm the connection.'
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter className='justify-between gap-4'>
          <p className='text-muted-foreground text-sm'>
            {t('After confirmation, return to MyAgents to finish setup.')}
          </p>
          <Button onClick={signIn}>{t('Sign in to continue')}</Button>
        </CardFooter>
      </Card>
    )
  } else if (optionsQuery.isLoading) {
    content = <LoadingState message={t('Loading connection options...')} />
  } else if (optionsQuery.isError || !optionsQuery.data?.success) {
    content = (
      <ErrorState
        title={t('The connection request could not be loaded.')}
        onRetry={() => void optionsQuery.refetch()}
      />
    )
  } else if (groups.length === 0) {
    content = (
      <Card>
        <CardHeader>
          <CardTitle>
            {t('No eligible models are currently available for this account.')}
          </CardTitle>
          <CardDescription>
            {t(
              'Contact the site administrator if you need access to a model group.'
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter className='justify-end'>
          <Button
            variant='outline'
            onClick={() => cancelMutation.mutate()}
            disabled={cancelMutation.isPending}
          >
            {t('Cancel connection')}
          </Button>
        </CardFooter>
      </Card>
    )
  } else {
    content = (
      <Card>
        <CardHeader>
          <CardTitle>{t('Connection setup')}</CardTitle>
          <CardDescription>
            {t('Choose the group and model your agent can use.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='grid gap-5 sm:grid-cols-2'>
          <label className='grid gap-2 text-sm font-medium'>
            {t('Select a group')}
            <NativeSelect
              value={group}
              onChange={(event) => setGroup(event.target.value)}
              className='w-full'
            >
              {groups.map((item) => (
                <NativeSelectOption key={item.id} value={item.id}>
                  {item.description || item.id}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </label>
          <label className='grid gap-2 text-sm font-medium'>
            {t('Select a model')}
            <NativeSelect
              value={model}
              onChange={(event) => setModel(event.target.value)}
              className='w-full'
              disabled={models.length === 0}
            >
              {models.map((item) => (
                <NativeSelectOption key={item} value={item}>
                  {item}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </label>
        </CardContent>
        <CardFooter className='justify-end gap-3'>
          <Button
            variant='outline'
            onClick={() => cancelMutation.mutate()}
            disabled={authorizeMutation.isPending || cancelMutation.isPending}
          >
            {t('Cancel connection')}
          </Button>
          <Button
            onClick={() => authorizeMutation.mutate()}
            disabled={!group || !model || authorizeMutation.isPending}
          >
            {t('Confirm and continue')}
          </Button>
        </CardFooter>
      </Card>
    )
  }

  return (
    <div className='bg-muted/40 min-h-svh py-10 sm:py-16'>
      <main className='container mx-auto grid max-w-5xl gap-6 px-4 sm:px-6'>
        <section className='space-y-3 text-center sm:text-left'>
          <p className='text-primary text-sm font-medium'>
            {t('Connect MyAgents')}
          </p>
          <h1 className='text-3xl font-semibold tracking-tight'>
            {t('The One connection')}
          </h1>
          <p className='text-muted-foreground max-w-3xl text-sm sm:text-base'>
            {t(
              'This secure flow configures a provider, a read-only MCP server, and the The One Gateway Skill in MyAgents.'
            )}
          </p>
        </section>

        {content}

        <p className='text-muted-foreground text-center text-sm'>
          {t(
            'Use your system browser for this page. You always enter passwords and two-factor codes yourself; the agent never needs them in chat.'
          )}
        </p>
      </main>
    </div>
  )
}
