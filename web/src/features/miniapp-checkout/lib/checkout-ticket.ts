/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
const maxCheckoutTicketLength = 512
const miniAppCheckoutPath = '/miniapp-checkout'
export const miniAppCheckoutTicketBootstrapWindowKey =
  '__theOneMiniAppCheckoutTicket'

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
