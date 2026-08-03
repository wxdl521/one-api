/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import fs from 'node:fs/promises'
import path from 'node:path'

const localesDir = path.resolve('src/i18n/locales')

const newKeys = {
  en: {
    'The One product suite': 'The One Product Suite',
    'Three ways to make AI work for you': 'Three ways to make AI work for you',
    'Deploy OpenClaw and other AI agents':
      'Deploy OpenClaw and other AI agents',
    '¥210': '¥210',
    '2C4G · 40G': '2C4G · 40G',
    'ArkClaw compact server with AI agent nodes':
      'ArkClaw compact server with AI agent nodes',
    'Team edition': 'Team edition',
    '¥149 / seat / month': '¥149 / seat / month',
    'Starting from 1 seat': 'Starting from 1 seat',
    '¥40 model allowance / seat / month': '¥40 model allowance / seat / month',
    '30M Tokens / seat / month': '30M Tokens / seat / month',
    'IDE · plugins · Solo (desktop / web)':
      'IDE · plugins · Solo (desktop / web)',
    'Trae Work developer workstation': 'Trae Work developer workstation',
    'One key for multiple frontier models':
      'One key for multiple frontier models',
    Medium: 'Medium',
    '¥200 / month': '¥200 / month',
    '100,000 AFP monthly allowance': '100,000 AFP monthly allowance',
    '35,000 AFP weekly allowance': '35,000 AFP weekly allowance',
    '10,000 AFP five-hour allowance': '10,000 AFP five-hour allowance',
    'Agent Plan access key and credential':
      'Agent Plan access key and credential',
  },
  zh: {
    'The One product suite': 'The One 产品矩阵',
    'Three ways to make AI work for you': '三种方式，让 AI 真正为你工作',
    'Deploy OpenClaw and other AI agents': '部署 OpenClaw 等智能体',
    '¥210': '¥210',
    '2C4G · 40G': '2C4G · 40G',
    'ArkClaw compact server with AI agent nodes':
      'ArkClaw 紧凑服务器与 AI 智能体节点',
    'Team edition': '团队版',
    '¥149 / seat / month': '¥149 / 席 / 月',
    'Starting from 1 seat': '1 席起购',
    '¥40 model allowance / seat / month': '¥40 模型额度 / 席 / 月',
    '30M Tokens / seat / month': '30M Tokens / 席 / 月',
    'IDE · plugins · Solo (desktop / web)': 'IDE · 插件 · Solo（桌面 / Web）',
    'Trae Work developer workstation': 'Trae Work 开发工作台',
    'One key for multiple frontier models': '一把 Key，调用多种前沿模型',
    Medium: '中杯',
    '¥200 / month': '¥200 / 月',
    '100,000 AFP monthly allowance': '100,000 AFP 月额度',
    '35,000 AFP weekly allowance': '35,000 AFP 周额度',
    '10,000 AFP five-hour allowance': '10,000 AFP 5 小时额度',
    'Agent Plan access key and credential': 'Agent Plan 访问密钥与凭证',
  },
  'zh-TW': {
    'The One product suite': 'The One 產品矩陣',
    'Three ways to make AI work for you': '三種方式，讓 AI 真正為你工作',
    'Deploy OpenClaw and other AI agents': '部署 OpenClaw 等智慧體',
    '¥210': '¥210',
    '2C4G · 40G': '2C4G · 40G',
    'ArkClaw compact server with AI agent nodes':
      'ArkClaw 精巧伺服器與 AI 智慧體節點',
    'Team edition': '團隊版',
    '¥149 / seat / month': '¥149 / 席 / 月',
    'Starting from 1 seat': '1 席起購',
    '¥40 model allowance / seat / month': '¥40 模型額度 / 席 / 月',
    '30M Tokens / seat / month': '30M Tokens / 席 / 月',
    'IDE · plugins · Solo (desktop / web)': 'IDE · 外掛 · Solo（桌面 / Web）',
    'Trae Work developer workstation': 'Trae Work 開發工作站',
    'One key for multiple frontier models': '一把 Key，呼叫多種前沿模型',
    Medium: '中杯',
    '¥200 / month': '¥200 / 月',
    '100,000 AFP monthly allowance': '100,000 AFP 月額度',
    '35,000 AFP weekly allowance': '35,000 AFP 週額度',
    '10,000 AFP five-hour allowance': '10,000 AFP 5 小時額度',
    'Agent Plan access key and credential': 'Agent Plan 存取金鑰與憑證',
  },
  fr: {
    'The One product suite': 'Suite de produits The One',
    'Three ways to make AI work for you':
      'Trois façons de faire travailler l’IA pour vous',
    'Deploy OpenClaw and other AI agents':
      'Déployez OpenClaw et d’autres agents IA',
    '¥210': '¥210',
    '2C4G · 40G': '2C4G · 40G',
    'ArkClaw compact server with AI agent nodes':
      'Serveur compact ArkClaw avec nœuds d’agents IA',
    'Team edition': 'Édition équipe',
    '¥149 / seat / month': '¥149 / siège / mois',
    'Starting from 1 seat': 'À partir de 1 siège',
    '¥40 model allowance / seat / month': '¥40 de crédit modèle / siège / mois',
    '30M Tokens / seat / month': '30M de tokens / siège / mois',
    'IDE · plugins · Solo (desktop / web)':
      'IDE · extensions · Solo (bureau / web)',
    'Trae Work developer workstation': 'Poste de développement Trae Work',
    'One key for multiple frontier models':
      'Une clé pour plusieurs modèles de pointe',
    Medium: 'Intermédiaire',
    '¥200 / month': '¥200 / mois',
    '100,000 AFP monthly allowance': '100 000 AFP par mois',
    '35,000 AFP weekly allowance': '35 000 AFP par semaine',
    '10,000 AFP five-hour allowance': '10 000 AFP toutes les 5 heures',
    'Agent Plan access key and credential':
      'Clé d’accès et identifiant Agent Plan',
  },
  ru: {
    'The One product suite': 'Линейка продуктов The One',
    'Three ways to make AI work for you':
      'Три способа заставить ИИ работать на вас',
    'Deploy OpenClaw and other AI agents':
      'Развертывайте OpenClaw и других ИИ-агентов',
    '¥210': '¥210',
    '2C4G · 40G': '2C4G · 40G',
    'ArkClaw compact server with AI agent nodes':
      'Компактный сервер ArkClaw с узлами ИИ-агентов',
    'Team edition': 'Командная версия',
    '¥149 / seat / month': '¥149 / место / месяц',
    'Starting from 1 seat': 'От 1 места',
    '¥40 model allowance / seat / month': '¥40 модельной квоты / место / месяц',
    '30M Tokens / seat / month': '30M токенов / место / месяц',
    'IDE · plugins · Solo (desktop / web)':
      'IDE · плагины · Solo (настольная / веб)',
    'Trae Work developer workstation': 'Рабочая станция разработчика Trae Work',
    'One key for multiple frontier models':
      'Один ключ для множества передовых моделей',
    Medium: 'Средний',
    '¥200 / month': '¥200 / месяц',
    '100,000 AFP monthly allowance': '100 000 AFP в месяц',
    '35,000 AFP weekly allowance': '35 000 AFP в неделю',
    '10,000 AFP five-hour allowance': '10 000 AFP на 5 часов',
    'Agent Plan access key and credential':
      'Ключ доступа и учетные данные Agent Plan',
  },
  ja: {
    'The One product suite': 'The One 製品ラインアップ',
    'Three ways to make AI work for you': 'AI を本当に活かす、3 つの方法',
    'Deploy OpenClaw and other AI agents':
      'OpenClaw などの AI エージェントを導入',
    '¥210': '¥210',
    '2C4G · 40G': '2C4G · 40G',
    'ArkClaw compact server with AI agent nodes':
      'AI エージェントノードを備えた ArkClaw コンパクトサーバー',
    'Team edition': 'チーム版',
    '¥149 / seat / month': '¥149 / 席 / 月',
    'Starting from 1 seat': '1 席から購入可能',
    '¥40 model allowance / seat / month': '¥40 のモデル利用枠 / 席 / 月',
    '30M Tokens / seat / month': '30M トークン / 席 / 月',
    'IDE · plugins · Solo (desktop / web)':
      'IDE · プラグイン · Solo（デスクトップ / Web）',
    'Trae Work developer workstation': 'Trae Work 開発ワークステーション',
    'One key for multiple frontier models':
      '1 つのキーで複数の最先端モデルを利用',
    Medium: '中杯',
    '¥200 / month': '¥200 / 月',
    '100,000 AFP monthly allowance': '100,000 AFP の月間利用枠',
    '35,000 AFP weekly allowance': '35,000 AFP の週間利用枠',
    '10,000 AFP five-hour allowance': '10,000 AFP の5時間利用枠',
    'Agent Plan access key and credential':
      'Agent Plan のアクセスキーと認証情報',
  },
  vi: {
    'The One product suite': 'Bộ sản phẩm The One',
    'Three ways to make AI work for you':
      'Ba cách để AI thực sự làm việc cho bạn',
    'Deploy OpenClaw and other AI agents':
      'Triển khai OpenClaw và các tác tử AI khác',
    '¥210': '¥210',
    '2C4G · 40G': '2C4G · 40G',
    'ArkClaw compact server with AI agent nodes':
      'Máy chủ ArkClaw nhỏ gọn với các nút tác tử AI',
    'Team edition': 'Bản dành cho nhóm',
    '¥149 / seat / month': '¥149 / chỗ / tháng',
    'Starting from 1 seat': 'Từ 1 chỗ',
    '¥40 model allowance / seat / month': '¥40 hạn mức mô hình / chỗ / tháng',
    '30M Tokens / seat / month': '30M token / chỗ / tháng',
    'IDE · plugins · Solo (desktop / web)':
      'IDE · plugin · Solo (máy tính / web)',
    'Trae Work developer workstation':
      'Trạm làm việc cho lập trình viên Trae Work',
    'One key for multiple frontier models':
      'Một khóa cho nhiều mô hình tiên phong',
    Medium: 'Cỡ vừa',
    '¥200 / month': '¥200 / tháng',
    '100,000 AFP monthly allowance': '100.000 AFP mỗi tháng',
    '35,000 AFP weekly allowance': '35.000 AFP mỗi tuần',
    '10,000 AFP five-hour allowance': '10.000 AFP mỗi 5 giờ',
    'Agent Plan access key and credential':
      'Khóa truy cập và thông tin xác thực Agent Plan',
  },
}

const agentConnectEnglish = {
  'Connect MyAgents': 'Connect MyAgents',
  'The One connection': 'The One connection',
  'This secure flow configures a provider, a read-only MCP server, and the The One Gateway Skill in MyAgents.':
    'This secure flow configures a provider, a read-only MCP server, and the The One Gateway Skill in MyAgents.',
  'Start from the connector': 'Start from the connector',
  'The One connector needs a request ID. Download it and run one of these commands to start the connection.':
    'The One connector needs a request ID. Download it and run one of these commands to start the connection.',
  'Download connector for Windows': 'Download connector for Windows',
  'Download connector for macOS': 'Download connector for macOS',
  'Sign in to continue': 'Sign in to continue',
  'Sign in on this official page, choose a group and model, then confirm the connection.':
    'Sign in on this official page, choose a group and model, then confirm the connection.',
  'Your browser will return to the local connector.':
    'Your browser will return to the local connector.',
  'Loading connection options...': 'Loading connection options...',
  'The connection request could not be loaded.':
    'The connection request could not be loaded.',
  'No eligible models are currently available for this account.':
    'No eligible models are currently available for this account.',
  'Contact the site administrator if you need access to a model group.':
    'Contact the site administrator if you need access to a model group.',
  'Connection setup': 'Connection setup',
  'Choose the group and model MyAgents can use.':
    'Choose the group and model MyAgents can use.',
  'Select a group': 'Select a group',
  'Select a model': 'Select a model',
  'Confirm and continue': 'Confirm and continue',
  'Cancel connection': 'Cancel connection',
  'Failed to complete the connection.': 'Failed to complete the connection.',
  'Connection canceled.': 'Connection canceled.',
  'The connector only opens this page and receives its local callback. You always enter passwords and two-factor codes yourself.':
    'The connector only opens this page and receives its local callback. You always enter passwords and two-factor codes yourself.',
}

const agentConnectChinese = {
  'Connect MyAgents': '连接 MyAgents',
  'The One connection': 'The One 连接',
  'This secure flow configures a provider, a read-only MCP server, and the The One Gateway Skill in MyAgents.':
    '此安全流程将在 MyAgents 中配置模型供应商、只读 MCP 服务器和 The One Gateway Skill。',
  'Start from the connector': '请从连接器启动',
  'The One connector needs a request ID. Download it and run one of these commands to start the connection.':
    'The One 连接器需要请求 ID。下载后运行以下任一命令以开始连接。',
  'Download connector for Windows': '下载 Windows 连接器',
  'Download connector for macOS': '下载 macOS 连接器',
  'Sign in to continue': '登录以继续',
  'Sign in on this official page, choose a group and model, then confirm the connection.':
    '请在此官方页面登录，选择分组和模型，然后确认连接。',
  'Your browser will return to the local connector.':
    '浏览器将返回本地连接器。',
  'Loading connection options...': '正在加载连接选项…',
  'The connection request could not be loaded.': '无法加载连接请求。',
  'No eligible models are currently available for this account.':
    '此账户当前没有可用模型。',
  'Contact the site administrator if you need access to a model group.':
    '如需访问模型分组，请联系站点管理员。',
  'Connection setup': '连接设置',
  'Choose the group and model MyAgents can use.':
    '选择 MyAgents 可以使用的分组和模型。',
  'Select a group': '选择分组',
  'Select a model': '选择模型',
  'Confirm and continue': '确认并继续',
  'Cancel connection': '取消连接',
  'Failed to complete the connection.': '无法完成连接。',
  'Connection canceled.': '连接已取消。',
  'The connector only opens this page and receives its local callback. You always enter passwords and two-factor codes yourself.':
    '连接器仅打开此页面并接收本地回调。密码和双重验证码必须由你自行输入。',
}

const agentConnectTraditionalChinese = {
  ...agentConnectChinese,
  'Connect MyAgents': '連接 MyAgents',
  'The One connection': 'The One 連接',
  'This secure flow configures a provider, a read-only MCP server, and the The One Gateway Skill in MyAgents.':
    '此安全流程會在 MyAgents 中設定模型供應商、唯讀 MCP 伺服器和 The One Gateway Skill。',
  'Start from the connector': '請從連接器啟動',
  'The One connector needs a request ID. Download it and run one of these commands to start the connection.':
    'The One 連接器需要請求 ID。下載後執行以下任一命令以開始連接。',
  'Download connector for Windows': '下載 Windows 連接器',
  'Download connector for macOS': '下載 macOS 連接器',
  'Sign in to continue': '登入以繼續',
  'Sign in on this official page, choose a group and model, then confirm the connection.':
    '請在此官方頁面登入，選擇群組和模型，然後確認連接。',
  'Your browser will return to the local connector.':
    '瀏覽器將返回本機連接器。',
  'Loading connection options...': '正在載入連接選項…',
  'The connection request could not be loaded.': '無法載入連接請求。',
  'No eligible models are currently available for this account.':
    '此帳戶目前沒有可用模型。',
  'Contact the site administrator if you need access to a model group.':
    '如需存取模型群組，請聯絡網站管理員。',
  'Connection setup': '連接設定',
  'Choose the group and model MyAgents can use.':
    '選擇 MyAgents 可以使用的群組和模型。',
  'Select a group': '選擇群組',
  'Select a model': '選擇模型',
  'Confirm and continue': '確認並繼續',
  'Cancel connection': '取消連接',
  'Failed to complete the connection.': '無法完成連接。',
  'Connection canceled.': '連接已取消。',
  'The connector only opens this page and receives its local callback. You always enter passwords and two-factor codes yourself.':
    '連接器只會開啟此頁面並接收本機回呼。密碼和雙重驗證碼必須由你自行輸入。',
}

const productOrderTranslations = {
  en: {
    'Awaiting manual delivery': 'Awaiting manual delivery',
    'Awaiting payment confirmation': 'Awaiting payment confirmation',
    'Cancel order': 'Cancel order',
    Cancelled: 'Cancelled',
    'Confirm payment': 'Confirm payment',
    'Image uploaded': 'Image uploaded',
    'Mark as shipped': 'Mark as shipped',
    'My orders': 'My orders',
    'No orders yet': 'No orders yet',
    'Order number': 'Order number',
    'Pay by QR code': 'Pay by QR code',
    'Pay with wallet': 'Pay with wallet',
    'Payment instructions': 'Payment instructions',
    'Payment QR code': 'Payment QR code',
    'Payment QR code URL': 'Payment QR code URL',
    'Price in CNY cents': 'Price (CNY cents)',
    'Product Orders': 'Product Orders',
    'Scan to pay': 'Scan to pay',
    Shipped: 'Shipped',
  },
  zh: {
    'Awaiting manual delivery': '等待人工发货',
    'Awaiting payment confirmation': '等待确认付款',
    'Cancel order': '取消订单',
    Cancelled: '已取消',
    'Confirm payment': '确认收款',
    'Image uploaded': '图片已上传',
    'Mark as shipped': '标记为已发货',
    'My orders': '我的订单',
    'No orders yet': '暂无订单',
    'Order number': '订单号',
    'Pay by QR code': '扫码支付',
    'Pay with wallet': '使用钱包余额支付',
    'Payment instructions': '付款说明',
    'Payment QR code': '收款二维码',
    'Payment QR code URL': '收款二维码 URL',
    'Price in CNY cents': '价格（人民币分）',
    'Product Orders': '商品订单',
    'Scan to pay': '请扫码付款',
    Shipped: '已发货',
  },
  'zh-TW': {
    'Awaiting manual delivery': '等待人工出貨',
    'Awaiting payment confirmation': '等待確認付款',
    'Cancel order': '取消訂單',
    Cancelled: '已取消',
    'Confirm payment': '確認收款',
    'Image uploaded': '圖片已上傳',
    'Mark as shipped': '標記為已出貨',
    'My orders': '我的訂單',
    'No orders yet': '暫無訂單',
    'Order number': '訂單號',
    'Pay by QR code': '掃碼付款',
    'Pay with wallet': '使用錢包餘額付款',
    'Payment instructions': '付款說明',
    'Payment QR code': '收款 QR 碼',
    'Payment QR code URL': '收款 QR 碼 URL',
    'Price in CNY cents': '價格（人民幣分）',
    'Product Orders': '商品訂單',
    'Scan to pay': '請掃碼付款',
    Shipped: '已出貨',
  },
  fr: {
    'Awaiting manual delivery': 'En attente de livraison manuelle',
    'Awaiting payment confirmation': 'En attente de confirmation du paiement',
    'Cancel order': 'Annuler la commande',
    Cancelled: 'Annulée',
    'Confirm payment': 'Confirmer le paiement',
    'Image uploaded': 'Image téléversée',
    'Mark as shipped': 'Marquer comme expédiée',
    'My orders': 'Mes commandes',
    'No orders yet': 'Aucune commande',
    'Order number': 'Numéro de commande',
    'Pay by QR code': 'Payer par QR code',
    'Pay with wallet': 'Payer avec le portefeuille',
    'Payment instructions': 'Instructions de paiement',
    'Payment QR code': 'QR code de paiement',
    'Payment QR code URL': 'URL du QR code de paiement',
    'Price in CNY cents': 'Prix en centimes CNY',
    'Product Orders': 'Commandes produits',
    'Scan to pay': 'Scannez pour payer',
    Shipped: 'Expédiée',
  },
  ja: {
    'Awaiting manual delivery': '手動配送待ち',
    'Awaiting payment confirmation': '入金確認待ち',
    'Cancel order': '注文をキャンセル',
    Cancelled: 'キャンセル済み',
    'Confirm payment': '入金を確認',
    'Image uploaded': '画像をアップロードしました',
    'Mark as shipped': '発送済みにする',
    'My orders': '注文履歴',
    'No orders yet': '注文はまだありません',
    'Order number': '注文番号',
    'Pay by QR code': 'QRコードで支払う',
    'Pay with wallet': 'ウォレット残高で支払う',
    'Payment instructions': '支払い方法',
    'Payment QR code': '支払い用QRコード',
    'Payment QR code URL': '支払い用QRコードURL',
    'Price in CNY cents': '価格（人民元の分）',
    'Product Orders': '商品注文',
    'Scan to pay': 'スキャンして支払う',
    Shipped: '発送済み',
  },
  ru: {
    'Awaiting manual delivery': 'Ожидает ручной доставки',
    'Awaiting payment confirmation': 'Ожидает подтверждения оплаты',
    'Cancel order': 'Отменить заказ',
    Cancelled: 'Отменён',
    'Confirm payment': 'Подтвердить оплату',
    'Image uploaded': 'Изображение загружено',
    'Mark as shipped': 'Отметить как отправленный',
    'My orders': 'Мои заказы',
    'No orders yet': 'Заказов пока нет',
    'Order number': 'Номер заказа',
    'Pay by QR code': 'Оплатить по QR-коду',
    'Pay with wallet': 'Оплатить из кошелька',
    'Payment instructions': 'Инструкции по оплате',
    'Payment QR code': 'QR-код для оплаты',
    'Payment QR code URL': 'URL QR-кода для оплаты',
    'Price in CNY cents': 'Цена в центах CNY',
    'Product Orders': 'Заказы товаров',
    'Scan to pay': 'Отсканируйте для оплаты',
    Shipped: 'Отправлен',
  },
  vi: {
    'Awaiting manual delivery': 'Đang chờ giao thủ công',
    'Awaiting payment confirmation': 'Đang chờ xác nhận thanh toán',
    'Cancel order': 'Hủy đơn hàng',
    Cancelled: 'Đã hủy',
    'Confirm payment': 'Xác nhận thanh toán',
    'Image uploaded': 'Đã tải ảnh lên',
    'Mark as shipped': 'Đánh dấu đã giao',
    'My orders': 'Đơn hàng của tôi',
    'No orders yet': 'Chưa có đơn hàng',
    'Order number': 'Mã đơn hàng',
    'Pay by QR code': 'Thanh toán bằng mã QR',
    'Pay with wallet': 'Thanh toán bằng ví',
    'Payment instructions': 'Hướng dẫn thanh toán',
    'Payment QR code': 'Mã QR thanh toán',
    'Payment QR code URL': 'URL mã QR thanh toán',
    'Price in CNY cents': 'Giá theo xu CNY',
    'Product Orders': 'Đơn hàng sản phẩm',
    'Scan to pay': 'Quét để thanh toán',
    Shipped: 'Đã giao',
  },
}

for (const locale of Object.keys(newKeys)) {
  let translations = agentConnectEnglish
  if (locale === 'zh') {
    translations = agentConnectChinese
  } else if (locale === 'zh-TW') {
    translations = agentConnectTraditionalChinese
  }
  newKeys[locale] = {
    ...newKeys[locale],
    ...translations,
    ...productOrderTranslations[locale],
  }
}

async function main() {
  for (const [locale, translations] of Object.entries(newKeys)) {
    const filename = path.join(localesDir, `${locale}.json`)
    const raw = await fs.readFile(filename, 'utf8')
    const localeFile = JSON.parse(raw)
    localeFile.translation = {
      ...localeFile.translation,
      ...translations,
    }
    await fs.writeFile(filename, `${JSON.stringify(localeFile, null, 2)}\n`)
  }
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
