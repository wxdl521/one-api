import { clearPendingTextTestRequestID } from './pending-text-test'

export interface MiniAppSession {
  accessToken: string
  accessExpiresAt: number
  sid: string
}

let currentSession: MiniAppSession | null = null

export function setMiniAppSession(session: MiniAppSession): void {
  currentSession = {
    accessToken: session.accessToken.trim(),
    accessExpiresAt: session.accessExpiresAt,
    sid: session.sid.trim(),
  }
}

export function getMiniAppSession(): MiniAppSession | null {
  if (currentSession === null) {
    return null
  }
  if (currentSession.accessExpiresAt <= Math.floor(Date.now() / 1000)) {
    clearMiniAppSession()
    return null
  }
  return currentSession
}

export function clearMiniAppSession(): void {
  currentSession = null
  clearPendingTextTestRequestID()
}
