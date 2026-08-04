/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
const maxCheckoutTicketLength = 512
const miniAppCheckoutPath = '/miniapp-checkout'
const miniAppCheckoutTicketSessionStorageKey = '__theOneMiniAppCheckoutTicket'
export const miniAppCheckoutTicketBootstrapWindowKey =
  '__theOneMiniAppCheckoutTicket'

type SessionStorageLike = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export interface MiniAppCheckoutURLCapture {
  checkoutTicket: string | null
  visibleURL: string
}

export function createMiniAppCheckoutConfirmationPayload(
  checkoutTicket: string
): { checkout_ticket: string } | null {
  const normalizedTicket = checkoutTicket.trim()
  if (
    normalizedTicket.length === 0 ||
    normalizedTicket.length > maxCheckoutTicketLength
  ) {
    return null
  }
  return { checkout_ticket: normalizedTicket }
}

export function consumeMiniAppCheckoutBootstrapTicket(
  handoff: Record<string, unknown>
): string | null {
  const checkoutTicket = handoff[miniAppCheckoutTicketBootstrapWindowKey]
  delete handoff[miniAppCheckoutTicketBootstrapWindowKey]
  if (typeof checkoutTicket !== 'string') return null

  return (
    createMiniAppCheckoutConfirmationPayload(checkoutTicket)?.checkout_ticket ??
    null
  )
}

export function rememberMiniAppCheckoutTicket(
  checkoutTicket: string,
  storage: SessionStorageLike | null
): string | null {
  const payload = createMiniAppCheckoutConfirmationPayload(checkoutTicket)
  if (payload === null) return null
  if (storage === null) return payload.checkout_ticket
  try {
    storage.setItem(
      miniAppCheckoutTicketSessionStorageKey,
      payload.checkout_ticket
    )
  } catch {
    // Continue in memory when restrictive browser storage rejects writes.
  }
  return payload.checkout_ticket
}

export function getRememberedMiniAppCheckoutTicket(
  storage: SessionStorageLike | null
): string | null {
  if (storage === null) return null
  try {
    const checkoutTicket = storage.getItem(
      miniAppCheckoutTicketSessionStorageKey
    )
    return checkoutTicket
      ? (createMiniAppCheckoutConfirmationPayload(checkoutTicket)
          ?.checkout_ticket ?? null)
      : null
  } catch {
    return null
  }
}

export function clearRememberedMiniAppCheckoutTicket(
  storage: SessionStorageLike | null
): void {
  if (storage === null) return
  try {
    storage.removeItem(miniAppCheckoutTicketSessionStorageKey)
  } catch {
    // Storage cleanup is best-effort; the server still enforces one-time use.
  }
}

export function consumeMiniAppCheckoutURL(
  rawURL: string
): MiniAppCheckoutURLCapture | null {
  let url: URL
  try {
    url = new URL(rawURL, 'https://console.invalid')
  } catch {
    return null
  }
  if (url.pathname !== miniAppCheckoutPath) return null

  const visibleURL = url.pathname
  if (url.search !== '') return { checkoutTicket: null, visibleURL }

  const fragment = new URLSearchParams(url.hash.slice(1))
  const checkoutTickets = fragment.getAll('checkout_ticket')
  if (
    checkoutTickets.length !== 1 ||
    fragment.size !== 1 ||
    createMiniAppCheckoutConfirmationPayload(checkoutTickets[0]) === null
  ) {
    return { checkoutTicket: null, visibleURL }
  }
  return { checkoutTicket: checkoutTickets[0].trim(), visibleURL }
}
