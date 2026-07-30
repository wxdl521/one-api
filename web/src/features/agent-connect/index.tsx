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
import { useMutation, useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
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
import { useAuthStore } from '@/stores/auth-store'

import {
  authorizeAgentConnect,
  cancelAgentConnect,
  getAgentConnectOptions,
  type AgentConnectGroup,
} from './api'

const RELEASE_BASE_URL =
  'https://github.com/QuantumNous/the-one/releases/latest/download'
const EMPTY_GROUPS: AgentConnectGroup[] = []
const EMPTY_MODELS: string[] = []

type AgentConnectPageProps = {
  requestID?: string
}

function downloadCommand(platform: 'windows' | 'macos', origin: string) {
  if (platform === 'windows') {
    return `$p = Join-Path $env:TEMP 'the-one-connect.exe'; Invoke-WebRequest -Uri '${RELEASE_BASE_URL}/the-one-connect-windows-amd64.exe' -OutFile $p; & $p myagents --base-url '${origin}'`
  }
  return `arch="$(uname -m)"; asset="the-one-connect-darwin-amd64"; if [ "$arch" = "arm64" ]; then asset="the-one-connect-darwin-arm64"; fi; curl -fsSLo /tmp/the-one-connect "${RELEASE_BASE_URL}/$asset"; chmod +x /tmp/the-one-connect; /tmp/the-one-connect myagents --base-url '${origin}'`
}

export function AgentConnectPage({ requestID }: AgentConnectPageProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const auth = useAuthStore((state) => state.auth)
  const [group, setGroup] = useState('')
  const [model, setModel] = useState('')
  const authenticated = Boolean(auth.user && auth.accessToken)
  const origin = typeof window === 'undefined' ? '' : window.location.origin
  const windowsCommand = useMemo(
    () => downloadCommand('windows', origin),
    [origin]
  )
  const macOSCommand = useMemo(() => downloadCommand('macos', origin), [origin])
  const optionsQuery = useQuery({
    queryKey: ['agent-connect-options', requestID],
    queryFn: () => getAgentConnectOptions(requestID ?? ''),
    enabled: authenticated && Boolean(requestID),
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
      if (!response.success || !response.data?.callback_url) {
        toast.error(response.message || t('Failed to complete the connection.'))
        return
      }
      window.location.assign(response.data.callback_url)
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
  if (!requestID) {
    content = (
      <Card>
        <CardHeader>
          <CardTitle>{t('Start from the connector')}</CardTitle>
          <CardDescription>
            {t(
              'The One connector needs a request ID. Download it and run one of these commands to start the connection.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='grid gap-4'>
          <DownloadCommand
            label={t('Download connector for Windows')}
            value={windowsCommand}
          />
          <DownloadCommand
            label={t('Download connector for macOS')}
            value={macOSCommand}
          />
        </CardContent>
      </Card>
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
            {t('Your browser will return to the local connector.')}
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
            {t('Choose the group and model MyAgents can use.')}
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
            'The connector only opens this page and receives its local callback. You always enter passwords and two-factor codes yourself.'
          )}
        </p>
      </main>
    </div>
  )
}

function DownloadCommand({ label, value }: { label: string; value: string }) {
  return (
    <div className='grid gap-2'>
      <p className='text-sm font-medium'>{label}</p>
      <div className='bg-muted flex items-start gap-2 rounded-lg p-3'>
        <code className='min-w-0 flex-1 overflow-x-auto text-xs leading-5 break-all'>
          {value}
        </code>
        <CopyButton value={value} tooltip={label} />
      </div>
    </div>
  )
}
