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
import { api } from '@/lib/api'

export type AgentConnectGroup = {
  id: string
  description: string
  models: string[]
}

type AgentConnectResponse<T = undefined> = {
  success: boolean
  message?: string
  data?: T
}

export async function beginAgentConnectReauthentication(requestID: string) {
  const response = await api.post<
    AgentConnectResponse<{ reauthentication_required: boolean }>
  >(
    `/api/agent-connect/requests/${encodeURIComponent(requestID)}/reauthenticate`,
    undefined,
    { skipAuthRefresh: true, skipErrorHandler: true }
  )
  return response.data
}

export async function getAgentConnectOptions(requestID: string) {
  const response = await api.get<
    AgentConnectResponse<{ groups: AgentConnectGroup[] }>
  >(`/api/agent-connect/requests/${encodeURIComponent(requestID)}`)
  return response.data
}

export async function authorizeAgentConnect(
  requestID: string,
  input: { group: string; model: string }
) {
  const response = await api.post<
    AgentConnectResponse<{ callback_url?: string; completed?: boolean }>
  >(
    `/api/agent-connect/requests/${encodeURIComponent(requestID)}/authorize`,
    input
  )
  return response.data
}

export async function cancelAgentConnect(requestID: string) {
  const response = await api.post<AgentConnectResponse>(
    `/api/agent-connect/requests/${encodeURIComponent(requestID)}/cancel`
  )
  return response.data
}
