export type BindingFailureKey = 'bindingExpired' | 'bindingFailed' | 'bindingWebViewFailed'

export interface BindingLifecycleDependencies {
  createBinding: () => Promise<{ bindingId: string; webUrl: string }>
  getStatus: (bindingId: string) => Promise<'pending' | 'bound' | 'expired'>
  onBound: () => void
  onError: (key: BindingFailureKey) => void
  onLoading: () => void
  onReady: (webUrl: string) => void
  now?: () => number
  setInterval?: (callback: () => void, delay: number) => ReturnType<typeof globalThis.setInterval>
  clearInterval?: (timer: ReturnType<typeof globalThis.setInterval>) => void
  setTimeout?: (callback: () => void, delay: number) => ReturnType<typeof globalThis.setTimeout>
  clearTimeout?: (timer: ReturnType<typeof globalThis.setTimeout>) => void
}

const bindingPollIntervalMs = 3_000
const bindingTimeoutMs = 5 * 60 * 1_000

export class BindingLifecycle {
  private readonly dependencies: Required<Pick<BindingLifecycleDependencies,
    'createBinding' | 'getStatus' | 'onBound' | 'onError' | 'onLoading' | 'onReady'>> &
    Required<Pick<BindingLifecycleDependencies,
      'now' | 'setInterval' | 'clearInterval' | 'setTimeout' | 'clearTimeout'>>
  private bindingId: string | null = null
  private deadline = 0
  private deadlineTimer: ReturnType<typeof globalThis.setTimeout> | null = null
  private foreground = false
  private intervalTimer: ReturnType<typeof globalThis.setInterval> | null = null
  private polling = false
  private starting = false
  private stopped = false

  constructor(dependencies: BindingLifecycleDependencies) {
    this.dependencies = {
      ...dependencies,
      now: dependencies.now ?? Date.now,
      setInterval: dependencies.setInterval ?? globalThis.setInterval,
      clearInterval: dependencies.clearInterval ?? globalThis.clearInterval,
      setTimeout: dependencies.setTimeout ?? globalThis.setTimeout,
      clearTimeout: dependencies.clearTimeout ?? globalThis.clearTimeout,
    }
  }

  async show(): Promise<void> {
    if (this.stopped) {
      return
    }
    this.foreground = true
    if (this.bindingId !== null) {
      this.startPolling()
      return
    }
    if (this.starting) {
      return
    }
    this.starting = true
    this.dependencies.onLoading()
    try {
      const binding = await this.dependencies.createBinding()
      if (!this.isActive()) {
        return
      }
      this.bindingId = binding.bindingId
      this.deadline = this.dependencies.now() + bindingTimeoutMs
      this.deadlineTimer = this.dependencies.setTimeout(() => this.expire(), bindingTimeoutMs)
      this.dependencies.onReady(binding.webUrl)
      this.startPolling()
    } catch {
      if (this.isActive()) {
        this.fail('bindingFailed')
      }
    } finally {
      this.starting = false
    }
  }

  hide(): void {
    this.foreground = false
    this.stopPolling()
  }

  unload(): void {
    this.foreground = false
    this.stop()
  }

  cancel(): void {
    this.stop()
  }

  webViewFailed(): void {
    if (!this.stopped) {
      this.fail('bindingWebViewFailed')
    }
  }

  private isActive(): boolean {
    return !this.stopped && this.foreground
  }

  private startPolling(): void {
    if (!this.isActive() || this.bindingId === null || this.intervalTimer !== null) {
      return
    }
    this.intervalTimer = this.dependencies.setInterval(() => {
      void this.poll()
    }, bindingPollIntervalMs)
    void this.poll()
  }

  private async poll(): Promise<void> {
    const bindingId = this.bindingId
    if (!this.isActive() || bindingId === null || this.polling) {
      return
    }
    this.polling = true
    try {
      if (this.dependencies.now() >= this.deadline) {
        this.expire()
        return
      }
      const status = await this.dependencies.getStatus(bindingId)
      if (!this.isActive()) {
        return
      }
      if (this.dependencies.now() >= this.deadline || status === 'expired') {
        this.expire()
        return
      }
      if (status === 'bound') {
        this.stop()
        this.dependencies.onBound()
      }
    } catch {
      if (this.isActive()) {
        this.fail('bindingFailed')
      }
    } finally {
      this.polling = false
    }
  }

  private expire(): void {
    if (!this.stopped) {
      this.stop()
      this.dependencies.onError('bindingExpired')
    }
  }

  private fail(key: BindingFailureKey): void {
    this.stop()
    this.dependencies.onError(key)
  }

  private stop(): void {
    this.stopped = true
    this.stopPolling()
    if (this.deadlineTimer !== null) {
      this.dependencies.clearTimeout(this.deadlineTimer)
      this.deadlineTimer = null
    }
  }

  private stopPolling(): void {
    if (this.intervalTimer !== null) {
      this.dependencies.clearInterval(this.intervalTimer)
      this.intervalTimer = null
    }
  }
}
