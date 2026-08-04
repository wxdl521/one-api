import Taro from '@tarojs/taro'

const pendingTextTestRequestIDStorageKey = 'miniapp.pending-text-test-request-id.v1'
const uuidV4Pattern = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

function isTextTestRequestID(value: unknown): value is string {
  return typeof value === 'string' && uuidV4Pattern.test(value)
}

export function getPendingTextTestRequestID(): string | null {
  try {
    const requestID = Taro.getStorageSync<unknown>(pendingTextTestRequestIDStorageKey)
    if (isTextTestRequestID(requestID)) {
      return requestID
    }
  } catch {
    return null
  }
  clearPendingTextTestRequestID()
  return null
}

export function setPendingTextTestRequestID(requestID: string | null): void {
  if (requestID === null) {
    clearPendingTextTestRequestID()
    return
  }
  if (!isTextTestRequestID(requestID)) {
    clearPendingTextTestRequestID()
    return
  }
  try {
    Taro.setStorageSync(pendingTextTestRequestIDStorageKey, requestID)
  } catch {
    // Persistence is best-effort; a running request remains available in memory.
  }
}

export function clearPendingTextTestRequestID(): void {
  try {
    Taro.removeStorageSync(pendingTextTestRequestIDStorageKey)
  } catch {
    // Storage can be unavailable during process shutdown.
  }
}
