function getConfiguredPublicConsoleOrigin(): URL | null {
  const configuredOrigin = typeof __MINIAPP_BINDING_ORIGIN__ === 'string'
    ? __MINIAPP_BINDING_ORIGIN__.trim()
    : ''

  try {
    const origin = new URL(configuredOrigin)
    if (
      origin.protocol !== 'https:' ||
      origin.hostname === '' ||
      origin.username !== '' ||
      origin.password !== '' ||
      origin.pathname !== '/' ||
      origin.search !== '' ||
      origin.hash !== ''
    ) {
      return null
    }
    return origin
  } catch {
    return null
  }
}

export function getMiniAppUserAgreementURL(): string | null {
  const origin = getConfiguredPublicConsoleOrigin()
  return origin === null ? null : new URL('/user-agreement', origin).toString()
}
