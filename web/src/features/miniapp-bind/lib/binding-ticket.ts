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
const maxBindingTicketLength = 512
const miniAppBindPath = '/miniapp-bind'

export interface MiniAppBindingURLCapture {
  bindingTicket: string | null
  visibleURL: string
}

export function createMiniAppBindingConfirmationPayload(
  bindingTicket: string
): { binding_ticket: string } | null {
  const normalizedTicket = bindingTicket.trim()
  if (
    normalizedTicket.length === 0 ||
    normalizedTicket.length > maxBindingTicketLength
  ) {
    return null
  }
  return { binding_ticket: normalizedTicket }
}

export function consumeMiniAppBindingURL(
  rawURL: string
): MiniAppBindingURLCapture | null {
  let url: URL
  try {
    url = new URL(rawURL, 'https://console.invalid')
  } catch {
    return null
  }
  if (url.pathname !== miniAppBindPath) return null

  const visibleURL = url.pathname
  if (url.search !== '') return { bindingTicket: null, visibleURL }

  const fragment = new URLSearchParams(url.hash.slice(1))
  const bindingTickets = fragment.getAll('binding_ticket')
  if (
    bindingTickets.length !== 1 ||
    fragment.size !== 1 ||
    createMiniAppBindingConfirmationPayload(bindingTickets[0]) === null
  ) {
    return { bindingTicket: null, visibleURL }
  }
  return { bindingTicket: bindingTickets[0].trim(), visibleURL }
}
