import { miniAppStrings } from './i18n/strings'

const appConfig = {
  entryPagePath: 'pages/index/index',
  pages: [
    'pages/index/index',
    'pages/login/index',
    'pages/binding/index',
    'pages/account/index',
    'pages/tokens/index',
    'pages/products/index',
    'pages/orders/index',
    'pages/text-test/index',
  ],
  window: {
    navigationBarTitleText: miniAppStrings.serviceConnection,
    navigationBarBackgroundColor: '#ffffff',
    navigationBarTextStyle: 'black',
    backgroundColor: '#ffffff',
  },
}

export default appConfig
