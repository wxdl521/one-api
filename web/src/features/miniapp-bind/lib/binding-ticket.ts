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
export const miniAppBindPath = '/miniapp-bind'
export const miniAppBindingTicketBootstrapWindowKey =
  '__theOneMiniAppBindingTicket'
const miniAppBindingTicketSessionStorageKey =
  '__theOneMiniAppBindingTicketContinuation'

type SessionStorageLike = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export interface MiniAppBindingURLCapture {
  bindingTicket: string | null
  visibleURL: string
}

function getSessionStorage(): SessionStorageLike | null {
  if (typeof window === 'undefined') return null
  try {
    return window.sessionStorage
  } catch {
    return null
  }
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

export function consumeMiniAppBindingBootstrapTicket(
  handoff: Record<string, unknown>
): string | null {
  const bindingTicket = handoff[miniAppBindingTicketBootstrapWindowKey]
  delete handoff[miniAppBindingTicketBootstrapWindowKey]
  if (typeof bindingTicket !== 'string') return null

  return (
    createMiniAppBindingConfirmationPayload(bindingTicket)?.binding_ticket ??
    null
  )
}

// Keep the one-time binding credential only in tab-scoped storage while a
// signed-out browser completes password, OAuth, Passkey, or 2FA login. It is
// never placed in an auth redirect query parameter and is cleared once the
// server consumes it or rejects it as terminally invalid.
export function rememberMiniAppBindingSessionTicket(
  bindingTicket: string,
  storage: SessionStorageLike | null = getSessionStorage()
): boolean {
  const payload = createMiniAppBindingConfirmationPayload(bindingTicket)
  if (payload === null || storage === null) return false
  try {
    storage.setItem(
      miniAppBindingTicketSessionStorageKey,
      payload.binding_ticket
    )
    return true
  } catch {
    return false
  }
}

export function readMiniAppBindingSessionTicket(
  storage: SessionStorageLike | null = getSessionStorage()
): string | null {
  if (storage === null) return null
  try {
    const value = storage.getItem(miniAppBindingTicketSessionStorageKey)
    const payload =
      typeof value === 'string'
        ? createMiniAppBindingConfirmationPayload(value)
        : null
    if (payload !== null) return payload.binding_ticket
    storage.removeItem(miniAppBindingTicketSessionStorageKey)
  } catch {
    return null
  }
  return null
}

export function clearMiniAppBindingSessionTicket(
  storage: SessionStorageLike | null = getSessionStorage()
): void {
  if (storage === null) return
  try {
    storage.removeItem(miniAppBindingTicketSessionStorageKey)
  } catch {
    // Browser storage can be unavailable in restrictive WebViews.
  }
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
