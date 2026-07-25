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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { api } from '@/lib/api'

type ChatVideoTaskStatus = {
  task_id: string
  status: string
  progress: string
  fail_reason?: string
  content_url?: string
}

type ChatVideoTaskStatusResponse = {
  success: boolean
  data?: ChatVideoTaskStatus
  message?: string
}

type ChatVideoTaskPageProps = {
  taskId: string
  ticket: string
}

function isTerminalStatus(status: string | undefined): boolean {
  return status === 'SUCCESS' || status === 'FAILURE'
}

function statusLabel(status: string | undefined, t: (key: string) => string) {
  switch (status) {
    case 'SUCCESS':
      return t('Video ready')
    case 'FAILURE':
      return t('Video generation failed')
    case 'IN_PROGRESS':
      return t('Video generation in progress')
    default:
      return t('Video generation queued')
  }
}

export function ChatVideoTaskPage(props: ChatVideoTaskPageProps) {
  const { t } = useTranslation()
  const taskQuery = useQuery({
    queryKey: ['chat-video-task', props.taskId, props.ticket],
    queryFn: async () => {
      const response = await api.get<ChatVideoTaskStatusResponse>(
        `/api/chat-video/tasks/${encodeURIComponent(props.taskId)}`,
        {
          params: { ticket: props.ticket },
          skipBusinessError: true,
          skipErrorHandler: true,
        }
      )
      if (!response.data.success || !response.data.data) {
        throw new Error(response.data.message || t('Task not found'))
      }
      return response.data.data
    },
    refetchInterval: (query) =>
      isTerminalStatus(query.state.data?.status) ? false : 2000,
  })

  let content
  if (taskQuery.isLoading) {
    content = (
      <Card className='mx-auto max-w-3xl'>
        <CardContent className='text-muted-foreground py-10 text-center'>
          {t('Loading video task...')}
        </CardContent>
      </Card>
    )
  } else if (taskQuery.isError || !taskQuery.data) {
    content = (
      <Alert variant='destructive' className='mx-auto max-w-3xl'>
        <AlertTitle>{t('This task link is unavailable')}</AlertTitle>
        <AlertDescription>
          {t('The task may not exist, or this secure link has expired.')}
        </AlertDescription>
      </Alert>
    )
  } else {
    const task = taskQuery.data
    const terminal = isTerminalStatus(task.status)
    content = (
      <Card className='mx-auto max-w-3xl'>
        <CardHeader>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <CardTitle>{t('Video generation task')}</CardTitle>
            <Badge
              variant={task.status === 'FAILURE' ? 'destructive' : 'secondary'}
            >
              {statusLabel(task.status, t)}
            </Badge>
          </div>
          <CardDescription>
            {terminal
              ? t('This page no longer needs to refresh.')
              : t(
                  'This page refreshes automatically while your video is generating.'
                )}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-5'>
          {!terminal && task.progress ? (
            <p className='text-muted-foreground text-sm'>
              {t('Progress')}: {task.progress}
            </p>
          ) : null}

          {task.status === 'FAILURE' ? (
            <Alert variant='destructive'>
              <AlertTitle>{t('Video generation failed')}</AlertTitle>
              <AlertDescription>
                {task.fail_reason || t('Video generation failed')}
              </AlertDescription>
            </Alert>
          ) : null}

          {task.status === 'SUCCESS' && task.content_url ? (
            <div className='space-y-4'>
              <video
                controls
                className='bg-muted aspect-video w-full rounded-lg'
                src={task.content_url}
              >
                {t('Your browser does not support video playback.')}
              </video>
              <Button render={<a href={task.content_url} download />}>
                {t('Download video')}
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>
    )
  }

  return <PublicLayout>{content}</PublicLayout>
}
