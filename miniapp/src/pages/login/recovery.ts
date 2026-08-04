export type RegistrationRecovery = {
  action: 'bind-existing-account' | 'restart-login' | 'restart-verification'
  messageKey: 'registrationDisabled' | 'registrationTicketInvalid' | 'registrationVerificationRequired'
}

function getErrorCode(error: unknown): string | null {
  if (error !== null && typeof error === 'object' && typeof (error as { code?: unknown }).code === 'string') {
    return (error as { code: string }).code
  }
  return null
}

export function getRegistrationRecovery(error: unknown): RegistrationRecovery | null {
  switch (getErrorCode(error)) {
    case 'MINIAPP_REGISTRATION_DISABLED':
      return { action: 'bind-existing-account', messageKey: 'registrationDisabled' }
    case 'MINIAPP_TICKET_EXPIRED':
    case 'MINIAPP_TICKET_CONSUMED':
    case 'MINIAPP_PENDING_IDENTITY_UNAVAILABLE':
      return { action: 'restart-login', messageKey: 'registrationTicketInvalid' }
    case 'MINIAPP_EMAIL_VERIFICATION_REQUIRED':
    case 'MINIAPP_REGISTRATION_INVALID':
      return { action: 'restart-verification', messageKey: 'registrationVerificationRequired' }
    default:
      return null
  }
}
