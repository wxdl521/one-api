# The One 2.0 上线标准 (Release Standards)

> 状态: active | 版本: v1 | 落盘: 2026-08-03 | 分支: `release/2.0-productization`
> 作用: 定义 "可以上线" 的客观门禁。分三层——基线（通用）/ 前端 / 生产端。**任一层未过即不上线**。
> 关联: 修复任务见 `/PLAN.md`；部署执行见 `docs/release/DEPLOY-2.0-aliyun-sg.md`。

本标准全部锚定仓库已有事实（`AGENTS.md`、`web/AGENTS.md`、`.github/workflows/`），不是通用模板。每条门禁必须**机器可验证或有明确证据**才算过。

---

## 第 0 层 · 基线标准 (Baseline Gate)

所有交付物（后端/前端/桌面端/文档）通用，每个 PR 与每次发布都必过。

| # | 门禁 | 验证方式 (证据) | 阻断级 |
|---|------|----------------|--------|
| B1 | CI `pr-check` 绿 | GitHub Actions `pr-check.yml` 通过；PR 用仓库模板 `.github/PULL_REQUEST_TEMPLATE.md` 且 `Checklist` 段完整 | 🔴 |
| B2 | 受保护标识零改动 | grep diff 无对 `new-api` / `QuantumNous` / go module `github.com/QuantumNous/the-one` 的删改（AGENTS.md 治理红线） | 🔴 |
| B3 | AI 披露合规 | 非核心历史作者的 PR 正文以**散文**声明 AI 辅助；**禁带** `🤖 Generated with Claude Code`（`pr-check` anti-slop 的 `blocked-terms`，带则自动关 PR） | 🔴 |
| B4 | 分支隔离 | 全部 2.0 工作在 `release/2.0-productization`（基于 `codex/remote-desktop-installer`）；团队原分支/commit 保持不动，仅作基线参照 | 🔴 |
| B5 | 无密钥/本地库入库 | diff 无 `.env` / key / token / sqlite；`git ls-files` 无生产配置与凭据 | 🔴 |
| B6 | 构建可复现 | 后端 `go build` 成功；前端 `bun run build` 成功；桌面端 `node --check electron/main.js` 通过 | 🔴 |
| B7 | 提交规范 | conventional commits（feat/fix/docs/...）；无 "写一半" 半态 commit | 🟠 |

> B3 细则：仓库自身 CI 会自动关闭 "纯 AI slop" PR。我们的做法是——真实人工审核 + 按 AGENTS.md 用中文/英文散文说明 "AI 辅助生成、已人工验证"，绝不粘贴未经处理的 AI 输出，绝不带被 ban 的 footer 字符串。

---

## 第 1 层 · 前端上线标准 (Frontend Gate)

锚定 `web/AGENTS.md`。技术栈：React 19 + TypeScript + Rsbuild + Bun + Base UI + Tailwind。

| # | 门禁 | 验证方式 | 阻断级 |
|---|------|----------|--------|
| F1 | 生产构建干净 | `cd web && bun install --frozen-lockfile && bun run build` 无 error（release.yml 同款命令）；lockfile 不被改动 | 🔴 |
| F2 | i18n 完整 | `bun run i18n:sync` 无遗漏 key；7 语言（en/zh/zh-TW/fr/ru/ja/vi）无缺失；UI 文案全部走 `t('English key')`，无硬编码中文 | 🔴 |
| F3 | 无调试残留 | 生产码无 `console.log`（用现有 logger）；无 `debugger`、无临时 mock 数据 | 🟠 |
| F4 | 类型契约 | TypeScript 无新增 `any` 泛滥；API 请求走既有 typed 契约（web/AGENTS.md 3.6） | 🟠 |
| F5 | 错误态可见 | 关键流有 loading/error/empty 态（web/AGENTS.md 3.9）；无 unhandled promise rejection | 🟠 |
| F6 | 可访问性基线 | 交互元素可键盘可达、有 label（web/AGENTS.md 3.13）；不因样式吞掉焦点 | 🟠 |
| F7 | 前端安全 | 无 `dangerouslySetInnerHTML` 注入未净化内容；无把 token 落 localStorage 明文的新增路径（web/AGENTS.md 3.13） | 🔴 |
| F8 | 真实运行验证 | 关键用户流（登录、渠道管理、用量看板至少各一条）浏览器实测截图 + 无 console/network 报错（testing.md UI 门禁） | 🔴 |

---

## 第 2 层 · 生产端上线标准 (Production / Backend Gate)

锚定 `AGENTS.md` 后端红线 + 生产运维。技术栈：Go 1.22+ / Gin / GORM v2 / SQLite·MySQL·PostgreSQL / Redis。

### 2A. 代码正确性（AGENTS.md 硬规）

| # | 门禁 | 验证方式 | 阻断级 |
|---|------|----------|--------|
| P1 | JSON 包装 | 业务码零直接 `encoding/json` marshal/unmarshal，全走 `common.Marshal/Unmarshal/...`；`grep -rn "json.Marshal\|json.Unmarshal" --include=*.go` 仅剩类型引用 | 🔴 |
| P2 | 三库兼容 | 新增/改动 DB 码在 SQLite + MySQL≥5.7.8 + PostgreSQL≥9.6 全通过；行锁走 `lockForUpdate(tx)`，无 GORM v1 `gorm:query_option` 空锁；迁移用 `ADD COLUMN` 不用 `ALTER COLUMN` | 🔴 |
| P3 | 计费不变量 | 任何用户可控乘数（image n / video seconds / batch）上界校验；配额转换走 `common/quota_math.go` 的 `QuotaFrom*`/`QuotaRound`，无裸 `int(...)` cast；预扣+结算均不产生负计费；饱和事件走 `*Checked` + `attachQuotaSaturation` 落审计 | 🔴 |
| P4 | Relay DTO 语义 | 可选标量用指针 + `omitempty`，显式 0/false 不被吞；新 relay 格式的 max-tokens/count 从第一天起有上界 | 🔴 |
| P5 | 后端测试绿 | `go test ./...` 通过；计费/边界回归测试用 `testify/require`+`assert`，无凑覆盖率的假测试（AGENTS.md 测试质量） | 🔴 |
| P6 | i18n 后端 | 用户可见后端文案走 `nicksnyder/go-i18n/v2`（en/zh）；无硬编码 | 🟠 |

### 2B. 生产运维就绪

| # | 门禁 | 验证方式 | 阻断级 |
|---|------|----------|--------|
| P7 | 配置外置 | DB/Redis/密钥全走环境变量或 gitignored 配置；仓库零明文凭据；`SESSION_SECRET`、`CRYPTO_SECRET` 生产值已设且非默认 | 🔴 |
| P8 | 数据持久与备份 | 生产库（MySQL/PG）+ Redis 就位；**上线前有一次可恢复的 DB 备份**；数据目录/卷持久化 | 🔴 |
| P9 | 健康门禁 | `/api/status` 返 200 且 JSON 正常；反代 TLS 证书有效期 >30 天；容器/进程有 restart 策略 | 🔴 |
| P10 | 限流与滥用防护 | rate limit 中间件生效；管理接口鉴权正确；无调试后门 token（对照历史 F22 后门删除教训） | 🔴 |
| P11 | 可观测性 | 关键链路日志（model/参数/错误原因/用量）可查；错误可定位到 request | 🟠 |
| P12 | 回滚可行 | 上一版本镜像/二进制可一键回退；域名切换可回切；有回滚 runbook | 🔴 |

---

## 上线判定 (Go / No-Go)

- **Go 条件**：三层全部 🔴 门禁通过 + 🟠 门禁无未记录的例外 + 全局 code review（`/PLAN.md` M3）无遗留 CRITICAL/HIGH。
- **每个 🔴 例外**必须有帅老登书面拍板 + 记录残留风险，否则默认 No-Go。
- 证据留档：CI run 链接、构建输出、截图、smoke 输出统一贴进 `/PLAN.md` 对应 task 的验证摘要。

## 变更记录
- v1 (2026-08-03): 初版，锚定 AGENTS.md / web/AGENTS.md / CI 现状。
