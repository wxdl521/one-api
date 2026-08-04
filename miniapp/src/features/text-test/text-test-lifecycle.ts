import type { MiniTextTestStatus, StartMiniTextTestInput } from './text-test-service'
import {
  getPendingTextTestRequestID,
  setPendingTextTestRequestID,
} from '../../lib/pending-text-test'

export type { MiniTextTestStatus } from './text-test-service'

const pollDelayMilliseconds = 1_500
const foregroundPollLimitMilliseconds = 20_000
const inFlightStartRequestIDs = new Set<string>()

export interface TextTestLifecycleOptions {
  createRequestID: () => Promise<string>
  start: (input: StartMiniTextTestInput) => Promise<MiniTextTestStatus>
  getStatus: (requestID: string) => Promise<MiniTextTestStatus>
  onStatus: (status: MiniTextTestStatus) => void
  onPending: (requestID: string, key: 'textTestPending') => void
  onError: (error: unknown) => void
  onRequestIDChange?: (requestID: string | null) => void
  onStartChange?: (starting: boolean) => void
  getPersistedRequestID?: () => string | null
  setPersistedRequestID?: (requestID: string | null) => void
  isRetryableError?: (error: unknown) => boolean
  now?: () => number
  setTimeout?: (callback: () => void, delay: number) => ReturnType<typeof globalThis.setTimeout>
  clearTimeout?: (timer: ReturnType<typeof globalThis.setTimeout>) => void
}

export class TextTestLifecycle {
  private active = true
  private foreground = false
  private foregroundDeadline = 0
  private operation = 0
  private pendingRequestID: string | null = null
  private starting = false
  private pollTimer: ReturnType<typeof globalThis.setTimeout> | null = null
  private readonly clearTimeout: (timer: ReturnType<typeof globalThis.setTimeout>) => void
  private readonly now: () => number
  private readonly setTimeout: (callback: () => void, delay: number) => ReturnType<typeof globalThis.setTimeout>

  constructor(private readonly options: TextTestLifecycleOptions) {
    this.clearTimeout = options.clearTimeout ?? globalThis.clearTimeout
    this.now = options.now ?? Date.now
    this.setTimeout = options.setTimeout ?? globalThis.setTimeout
    this.pendingRequestID = (options.getPersistedRequestID ?? getPendingTextTestRequestID)()
  }

  getPendingRequestID(): string | null {
    return this.pendingRequestID
  }

  show(): void {
    if (!this.active) {
      return
    }
    this.foreground = true
    if (this.pendingRequestID !== null) {
      this.foregroundDeadline = this.now() + foregroundPollLimitMilliseconds
      void this.checkStatus()
    }
  }

  hide(): void {
    this.foreground = false
    this.operation += 1
    this.clearPollTimer()
  }

  unload(): void {
    this.active = false
    this.hide()
    this.foregroundDeadline = 0
  }

  resetSession(): void {
    this.unload()
    this.clearPendingRequestID()
  }

  async submit(input: Omit<StartMiniTextTestInput, 'clientRequestID'>): Promise<void> {
    if (!this.active || !this.foreground || this.starting) {
      return
    }
    if (this.pendingRequestID !== null) {
      this.foregroundDeadline = this.now() + foregroundPollLimitMilliseconds
      await this.checkStatus()
      return
    }

    this.setStarting(true)
    const operation = ++this.operation
    let requestID: string | null = null
    try {
      requestID = await this.options.createRequestID()
      if (!this.current(operation)) {
        return
      }
      this.pendingRequestID = requestID
      this.setPersistedRequestID(requestID)
      this.options.onRequestIDChange?.(requestID)
      this.foregroundDeadline = this.now() + foregroundPollLimitMilliseconds
      inFlightStartRequestIDs.add(requestID)
      const status = await this.options.start({ ...input, clientRequestID: requestID })
      if (!this.current(operation)) {
        return
      }
      this.handleStatus(status)
    } catch (error) {
      if (!this.current(operation)) {
        return
      }
      const retryable = this.options.isRetryableError?.(error) ?? true
      if (!retryable) {
        this.clearPendingRequestID()
      }
      this.options.onError(error)
      if (retryable) {
        this.schedulePoll()
      }
    } finally {
      if (requestID !== null) {
        inFlightStartRequestIDs.delete(requestID)
      }
      this.setStarting(false)
      if (!this.current(operation) && this.active && this.foreground && this.pendingRequestID !== null) {
        void this.checkStatus()
      }
    }
  }

  async checkStatus(): Promise<void> {
    const requestID = this.pendingRequestID
    if (!this.active || !this.foreground || requestID === null) {
      return
    }
    if (this.starting || inFlightStartRequestIDs.has(requestID)) {
      this.schedulePoll()
      return
    }
    this.clearPollTimer()
    if (this.now() >= this.foregroundDeadline) {
      this.options.onPending(requestID, 'textTestPending')
      return
    }

    const operation = ++this.operation
    try {
      const status = await this.options.getStatus(requestID)
      if (!this.current(operation) || this.pendingRequestID !== requestID) {
        return
      }
      this.handleStatus(status)
    } catch (error) {
      if (!this.current(operation)) {
        return
      }
      const retryable = this.options.isRetryableError?.(error) ?? true
      if (!retryable) {
        this.clearPendingRequestID()
      }
      this.options.onError(error)
      if (retryable) {
        this.schedulePoll()
      }
    }
  }

  private current(operation: number): boolean {
    return this.active && this.foreground && operation === this.operation
  }

  private handleStatus(status: MiniTextTestStatus): void {
    this.options.onStatus(status)
    if (status.state === 'running') {
      this.schedulePoll()
      return
    }
    this.clearPendingRequestID()
    this.foregroundDeadline = 0
    this.clearPollTimer()
  }

  private schedulePoll(): void {
    if (!this.active || !this.foreground || this.pendingRequestID === null) {
      return
    }
    this.clearPollTimer()
    if (this.now() >= this.foregroundDeadline) {
      this.options.onPending(this.pendingRequestID, 'textTestPending')
      return
    }
    this.pollTimer = this.setTimeout(() => {
      void this.checkStatus()
    }, pollDelayMilliseconds)
  }

  private clearPollTimer(): void {
    if (this.pollTimer === null) {
      return
    }
    this.clearTimeout(this.pollTimer)
    this.pollTimer = null
  }

  private clearPendingRequestID(): void {
    this.pendingRequestID = null
    this.setPersistedRequestID(null)
    this.options.onRequestIDChange?.(null)
  }

  private setStarting(starting: boolean): void {
    if (this.starting === starting) {
      return
    }
    this.starting = starting
    this.options.onStartChange?.(starting)
  }

  private setPersistedRequestID(requestID: string | null): void {
    const persistRequestID = this.options.setPersistedRequestID ?? setPendingTextTestRequestID
    persistRequestID(requestID)
  }
}
