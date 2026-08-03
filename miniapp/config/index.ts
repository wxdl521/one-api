const apiBaseUrl = process.env.TARO_APP_API_BASE_URL ?? ''

export default {
  projectName: 'miniapp',
  date: '2026-08-03',
  designWidth: 750,
  deviceRatio: {
    640: 2.34 / 2,
    750: 1,
    828: 1.81 / 2,
  },
  sourceRoot: 'src',
  outputRoot: 'dist',
  framework: 'react',
  compiler: 'webpack5',
  defineConstants: {
    __MINIAPP_API_BASE_URL__: JSON.stringify(apiBaseUrl),
  },
}
