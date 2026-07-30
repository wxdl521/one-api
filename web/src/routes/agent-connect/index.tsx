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
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { AgentConnectPage } from '@/features/agent-connect'

const searchSchema = z.object({
  request_id: z.string().min(1).max(128).optional(),
})

export const Route = createFileRoute('/agent-connect/')({
  validateSearch: searchSchema,
  component: AgentConnectRoute,
})

function AgentConnectRoute() {
  const { request_id: requestID } = Route.useSearch()
  return <AgentConnectPage requestID={requestID} />
}
