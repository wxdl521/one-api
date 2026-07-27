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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMemo } from 'react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Checkbox } from '@/components/ui/checkbox'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

const videoModels = [
  'doubao-seedance-1-0-pro-250528',
  'doubao-seedance-1-0-lite-t2v',
  'doubao-seedance-1-5-pro-251215',
  'doubao-seedance-2-0-260128',
  'doubao-seedance-2-0-fast-260128',
  'doubao-seedance-2.0',
  'veo-3.0-generate-001',
  'veo-3.0-fast-generate-001',
  'veo-3.1-generate-001',
  'veo-3.1-fast-generate-001',
  'veo-3.1-generate-preview',
  'veo-3.1-fast-generate-preview',
] as const

const chatVideoBridgeSchema = z.object({
  enabled: z.boolean(),
  models: z.array(z.enum(videoModels)),
  maxWaitSeconds: z.coerce.number().int().min(0).max(600),
  taskPageTTLSeconds: z.coerce.number().int().min(300).max(604800),
})

type ChatVideoBridgeFormValues = z.output<typeof chatVideoBridgeSchema>

type ChatVideoBridgeSectionProps = {
  defaultValues: {
    'chat_video_bridge.enabled': boolean
    'chat_video_bridge.models': string
    'chat_video_bridge.max_wait_seconds': number
    'chat_video_bridge.task_page_ttl_seconds': number
  }
}

function parseSelectedModels(raw: string): ChatVideoBridgeFormValues['models'] {
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(
      (value): value is ChatVideoBridgeFormValues['models'][number] =>
        typeof value === 'string' && videoModels.includes(value as never)
    )
  } catch {
    return []
  }
}

export function ChatVideoBridgeSection(props: ChatVideoBridgeSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const defaultValues = useMemo<ChatVideoBridgeFormValues>(
    () => ({
      enabled: props.defaultValues['chat_video_bridge.enabled'] ?? false,
      models: parseSelectedModels(
        props.defaultValues['chat_video_bridge.models']
      ),
      maxWaitSeconds:
        props.defaultValues['chat_video_bridge.max_wait_seconds'] ?? 300,
      taskPageTTLSeconds:
        props.defaultValues['chat_video_bridge.task_page_ttl_seconds'] ?? 86400,
    }),
    [props.defaultValues]
  )
  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<ChatVideoBridgeFormValues>({
      resolver: zodResolver(chatVideoBridgeSchema) as Resolver<
        ChatVideoBridgeFormValues,
        unknown,
        ChatVideoBridgeFormValues
      >,
      defaultValues,
      onSubmit: async (values, changedFields) => {
        if ('enabled' in changedFields) {
          await updateOption.mutateAsync({
            key: 'chat_video_bridge.enabled',
            value: values.enabled,
          })
        }
        if ('models' in changedFields) {
          await updateOption.mutateAsync({
            key: 'chat_video_bridge.models',
            value: JSON.stringify(values.models),
          })
        }
        if ('maxWaitSeconds' in changedFields) {
          await updateOption.mutateAsync({
            key: 'chat_video_bridge.max_wait_seconds',
            value: values.maxWaitSeconds,
          })
        }
        if ('taskPageTTLSeconds' in changedFields) {
          await updateOption.mutateAsync({
            key: 'chat_video_bridge.task_page_ttl_seconds',
            value: values.taskPageTTLSeconds,
          })
        }
      },
    })

  return (
    <>
      <FormNavigationGuard when={isDirty} />
      <SettingsSection title={t('Chat Video Bridge')}>
        <Form {...form}>
          <SettingsForm onSubmit={handleSubmit}>
            <SettingsPageFormActions
              onSave={handleSubmit}
              onReset={handleReset}
              isSaving={isSubmitting || updateOption.isPending}
              isResetDisabled={!isDirty}
            />
            <FormDirtyIndicator isDirty={isDirty} />

            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel htmlFor={field.name}>
                      {t('Enable Chat Video Bridge')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Convert eligible chat completion requests into supported video tasks without client changes.'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      id={field.name}
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <SettingsFormGrid>
              <FormItem className='lg:col-span-2'>
                <FormLabel>{t('Allowed Video Models')}</FormLabel>
                <FormDescription>
                  {t(
                    'Only checked models may use the chat video bridge. Start with one verified model for a safe rollout.'
                  )}
                </FormDescription>
                <div className='mt-3 grid gap-3 sm:grid-cols-2'>
                  {videoModels.map((model) => (
                    <FormField
                      key={model}
                      control={form.control}
                      name='models'
                      render={({ field }) => {
                        const selected = field.value.includes(model)
                        return (
                          <div className='flex items-center gap-2'>
                            <Checkbox
                              id={`chat-video-model-${model}`}
                              checked={selected}
                              onCheckedChange={(checked) => {
                                const nextModels = checked
                                  ? [...field.value, model]
                                  : field.value.filter((item) => item !== model)
                                field.onChange(nextModels)
                              }}
                            />
                            <label
                              htmlFor={`chat-video-model-${model}`}
                              className='cursor-pointer font-mono text-sm'
                            >
                              {model}
                            </label>
                          </div>
                        )
                      }}
                    />
                  ))}
                </div>
                <FormMessage />
              </FormItem>

              <FormField
                control={form.control}
                name='maxWaitSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Maximum Wait Time (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        max={600}
                        step={1}
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'The chat request waits for this long before returning a secure task page link. Set 0 to return immediately.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='taskPageTTLSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Task Page Link Validity (seconds)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={300}
                        max={604800}
                        step={1}
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'The signed task page and video download link expire after this time. The default is 24 hours.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGrid>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}
