export const miniAppStrings = {
  account: '账户',
  binding: '绑定账户',
  bindingExpired: '绑定已过期，请重新进入后再试。',
  bindingFailed: '账户绑定未完成，请重试。',
  bindingLoading: '正在准备安全绑定页面…',
  bindingPrompt: '请在打开的网页中登录并确认绑定。',
  bindingWebViewFailed: '绑定网页无法打开，请稍后重试。',
  connecting: '正在连接服务…',
  continueBinding: '绑定已有账户',
  login: '登录',
  loginFailed: '登录暂时不可用，请重试。',
  loginRequired: '需要重新验证微信身份。',
  password: '密码',
  register: '注册新账户',
  registrationDisabled: '当前不支持注册新账户，请绑定已有账户。',
  registerFailed: '注册暂时不可用，请重试。',
  registerPrompt: '没有账户？可直接创建一个。',
  registrationTicketInvalid: '本次身份验证已失效，请重新登录。',
  registrationVerificationRequired: '注册需要完成验证，请重新登录后继续。',
  restartLogin: '重新登录',
  restartVerification: '重新登录并验证',
  retry: '重试',
  serviceConnection: '服务连接',
  username: '用户名',
} as const

export type MiniAppStringKey = keyof typeof miniAppStrings

export function t(key: MiniAppStringKey): string {
  return miniAppStrings[key]
}
