import { describe, expect, it } from 'vitest'

import { getRegistrationRecovery } from './recovery'

describe('registration recovery UI', () => {
  it('offers existing-account binding when registration is disabled', () => {
    expect(getRegistrationRecovery({ code: 'MINIAPP_REGISTRATION_DISABLED' })).toEqual({
      action: 'bind-existing-account',
      messageKey: 'registrationDisabled',
    })
  })

  it.each(['MINIAPP_TICKET_EXPIRED', 'MINIAPP_TICKET_CONSUMED'])(
    'requires a fresh login instead of a blind retry for %s',
    (code) => {
      expect(getRegistrationRecovery({ code })).toEqual({
        action: 'restart-login',
        messageKey: 'registrationTicketInvalid',
      })
    },
  )

  it.each(['MINIAPP_EMAIL_VERIFICATION_REQUIRED', 'MINIAPP_REGISTRATION_INVALID'])(
    'offers verification restart for %s',
    (code) => {
      expect(getRegistrationRecovery({ code })).toEqual({
        action: 'restart-verification',
        messageKey: 'registrationVerificationRequired',
      })
    },
  )
})
