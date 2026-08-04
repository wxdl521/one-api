import { Text, View, WebView } from '@tarojs/components'
import { useState } from 'react'

import { getMiniAppUserAgreementURL } from '../../features/legal/user-agreement'
import { t } from '../../i18n/strings'

export default function UserAgreementPage() {
  const [failed, setFailed] = useState(false)
  const userAgreementURL = getMiniAppUserAgreementURL()

  if (userAgreementURL === null || failed) {
    return (
      <View className="user-agreement-shell">
        <Text>{t('userAgreementUnavailable')}</Text>
      </View>
    )
  }

  return <WebView src={userAgreementURL} onError={() => setFailed(true)} />
}
