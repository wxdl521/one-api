import { Text, View, WebView } from '@tarojs/components'
import { useUnload } from '@tarojs/taro'
import { useState } from 'react'

import { clearMiniAppCheckoutWebURL, getMiniAppCheckoutWebURL } from '../../features/commerce/commerce-service'
import { t } from '../../i18n/strings'

export default function CheckoutPage() {
  const [webURL] = useState(getMiniAppCheckoutWebURL)
  const [failed, setFailed] = useState(false)

  useUnload(() => {
    clearMiniAppCheckoutWebURL()
  })

  if (webURL === null || failed) {
    return (
      <View className="checkout-shell">
        <Text>{t(webURL === null ? 'checkoutExpired' : 'checkoutFailed')}</Text>
      </View>
    )
  }

  return <WebView src={webURL} onError={() => setFailed(true)} />
}
