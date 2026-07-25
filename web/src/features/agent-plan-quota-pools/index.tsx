/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  createAgentPlanQuotaPool,
  deleteAgentPlanQuotaPool,
  getAgentPlanEligibleSourceChannels,
  getAgentPlanQuotaPools,
  syncAgentPlanQuotaPool,
  updateAgentPlanQuotaPool,
} from '@/features/subscriptions/api'

import { getQuotaPoolStockValues } from './lib'
import type { AgentPlanEligibleSourceChannel, AgentPlanQuotaPool } from './types'

const AFP_MICROS = 1_000_000

function formatAFP(value: number): string {
  return (value / AFP_MICROS).toLocaleString(undefined, { maximumFractionDigits: 2 })
}

function formatTime(value: number, fallback: string): string {
  return value > 0 ? new Date(value * 1000).toLocaleString() : fallback
}

export function AgentPlanQuotaPools() {
  const { t } = useTranslation()
  const [pools, setPools] = useState<AgentPlanQuotaPool[]>([])
  const [channels, setChannels] = useState<AgentPlanEligibleSourceChannel[]>([])
  const [name, setName] = useState('')
  const [channelId, setChannelId] = useState('')
  const [multiplier, setMultiplier] = useState('1')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [poolResponse, channelResponse] = await Promise.all([
        getAgentPlanQuotaPools(),
        getAgentPlanEligibleSourceChannels(),
      ])
      setPools(poolResponse.success ? poolResponse.data || [] : [])
      setChannels(channelResponse.success ? channelResponse.data || [] : [])
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => { void load() }, [load])

  const create = async () => {
    if (!name.trim() || !channelId || Number(multiplier) <= 0) {
      toast.error(t('Complete all quota pool fields'))
      return
    }
    setSaving(true)
    try {
      const response = await createAgentPlanQuotaPool({ name: name.trim(), source_channel_id: Number(channelId), display_multiplier: Number(multiplier) })
      if (!response.success) {
        toast.error(response.message || t('Create failed'))
        return
      }
      setName('')
      setChannelId('')
      setMultiplier('1')
      toast.success(t('Create succeeded'))
      await load()
    } catch { toast.error(t('Request failed')) } finally { setSaving(false) }
  }

  const sync = async (pool: AgentPlanQuotaPool) => {
    try {
      const response = await syncAgentPlanQuotaPool(pool.id)
      if (!response.success) toast.error(response.message || t('Sync failed'))
      else { toast.success(t('Synced successfully')); await load() }
    } catch { toast.error(t('Request failed')) }
  }

  const remove = async (pool: AgentPlanQuotaPool) => {
    if (!window.confirm(t('Delete quota pool {{name}}?', { name: pool.name }))) return
    try {
      const response = await deleteAgentPlanQuotaPool(pool.id)
      if (!response.success) toast.error(response.message || t('Delete failed'))
      else { toast.success(t('Delete succeeded')); await load() }
    } catch { toast.error(t('Request failed')) }
  }

  const changeMultiplier = async (pool: AgentPlanQuotaPool, value: string) => {
    const next = Number(value)
    if (!Number.isFinite(next) || next <= 0 || next === pool.display_multiplier) return
    try {
      const response = await updateAgentPlanQuotaPool(pool.id, { display_multiplier: next })
      if (!response.success) toast.error(response.message || t('Update failed'))
      else await load()
    } catch { toast.error(t('Request failed')) }
  }

  const updatePool = async (pool: AgentPlanQuotaPool, data: { name?: string; source_channel_id?: number }) => {
    try {
      const response = await updateAgentPlanQuotaPool(pool.id, data)
      if (!response.success) toast.error(response.message || t('Update failed'))
      else await load()
    } catch { toast.error(t('Request failed')) }
  }

  let content: ReactNode = <p className='text-muted-foreground'>{t('Loading...')}</p>
  if (!loading && pools.length === 0) {
    content = <p className='text-muted-foreground'>{t('No quota pools yet')}</p>
  }
  if (!loading && pools.length > 0) {
    content = pools.map((pool) => {
      const stock = getQuotaPoolStockValues(pool)
      return <Card key={pool.id}>
      <CardHeader className='flex-row items-center justify-between gap-3'><div><CardTitle>{pool.name}</CardTitle><p className='text-muted-foreground mt-1 text-sm'>{t('Source channel')} #{pool.source_channel_id} · {pool.sync_status === 'available' ? t('Synchronized') : t('Sync unavailable')}</p></div><div className='flex gap-2'><Button variant='outline' size='sm' onClick={() => sync(pool)}><RefreshCw />{t('Sync')}</Button><Button variant='outline' size='sm' onClick={() => remove(pool)} disabled={pool.stock.active_reservation_afp_micros > 0}><Trash2 />{t('Delete')}</Button></div></CardHeader>
      <CardContent className='grid gap-4 text-sm sm:grid-cols-2 lg:grid-cols-4'>
        <div className='space-y-1'><Label>{t('Pool name')}</Label><Input defaultValue={pool.name} onBlur={(event) => { const value = event.target.value.trim(); if (value && value !== pool.name) void updatePool(pool, { name: value }) }} /></div>
        <div className='space-y-1'><Label>{t('Source channel')}</Label><Select value={String(pool.source_channel_id)} onValueChange={(value) => { if (value && Number(value) !== pool.source_channel_id) void updatePool(pool, { source_channel_id: Number(value) }) }}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{channels.map((channel) => <SelectItem key={channel.id} value={String(channel.id)}>{channel.name} #{channel.id}</SelectItem>)}</SelectContent></Select></div>
        <div><div className='text-muted-foreground'>{t('Official monthly AFP')}</div><strong>{formatAFP(pool.official_monthly_remaining_afp_micros)} AFP</strong></div>
        <div><div className='text-muted-foreground'>{t('5-hour / weekly AFP')}</div><strong>{formatAFP(pool.five_hour_remaining_afp_micros)} / {formatAFP(pool.weekly_remaining_afp_micros)} AFP</strong></div>
        <div><div className='text-muted-foreground'>{t('Reserved / available AFP')}</div><strong>{formatAFP(stock.reservedAFPMicros)} / {formatAFP(stock.sellableAFPMicros)} AFP</strong></div>
        <div className='space-y-1'><Label>{t('Display multiplier')}</Label><Input defaultValue={String(pool.display_multiplier)} type='number' min='0.000001' step='0.01' onBlur={(event) => void changeMultiplier(pool, event.target.value)} /></div>
        <div className='text-muted-foreground sm:col-span-2 lg:col-span-4'>{t('Last sync')}: {formatTime(pool.synced_at, t('Never'))} · {t('Monthly reset')}: {formatTime(pool.monthly_reset_at, t('Unknown'))}</div>
      </CardContent>
      </Card>
    })
  }

  return <SectionPageLayout fixedContent>
    <SectionPageLayout.Title>{t('Agent Plan quota pools')}</SectionPageLayout.Title>
    <SectionPageLayout.Content>
      <div className='space-y-4 overflow-auto pb-8'>
        <Card>
          <CardHeader><CardTitle>{t('Create quota pool')}</CardTitle></CardHeader>
          <CardContent className='grid gap-3 md:grid-cols-4'>
            <div className='space-y-1'><Label>{t('Pool name')}</Label><Input value={name} onChange={(event) => setName(event.target.value)} /></div>
            <div className='space-y-1'><Label>{t('Source channel')}</Label><Select value={channelId} onValueChange={(value) => setChannelId(value || '')}><SelectTrigger><SelectValue placeholder={t('Select a source channel')} /></SelectTrigger><SelectContent>{channels.map((channel) => <SelectItem key={channel.id} value={String(channel.id)}>{channel.name} #{channel.id}</SelectItem>)}</SelectContent></Select></div>
            <div className='space-y-1'><Label>{t('Display multiplier')}</Label><Input type='number' min='0.000001' step='0.01' value={multiplier} onChange={(event) => setMultiplier(event.target.value)} /></div>
            <div className='flex items-end'><Button onClick={create} disabled={saving}><Plus />{t('Create')}</Button></div>
          </CardContent>
        </Card>
        {content}
      </div>
    </SectionPageLayout.Content>
  </SectionPageLayout>
}
