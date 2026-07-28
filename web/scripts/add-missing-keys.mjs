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
