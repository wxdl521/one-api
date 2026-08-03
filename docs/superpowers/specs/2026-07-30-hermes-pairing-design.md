# Hermes Skill Pairing Design

## Goal

Let a user send Hermes one instruction that reads an official The One Skill,
opens the existing browser authorization page, and configures a non-default
The One provider, read-only MCP, and usage Skill. The user only operates the
login, two-factor authentication, group/model selection, and confirmation.

## User-facing contract

```text
请阅读并执行 https://the-one.bolierxiang.cn/skills/hermes/SKILL.md，为我接入 The One：保持当前默认模型不变；打开浏览器让我自行登录并确认分组与模型；随后自动配置独立 The One 供应商、只读 MCP 和 the-one-gateway Skill。不得读取、打印、保存或要求我粘贴 API Key、密码或 2FA。
```

## Architecture

Hermes uses the existing pairing authorization state machine, adding the
explicit client kind `hermes-skill`. It retains the existing PKCE verifier in
agent memory, waits for browser authorization, and exchanges it once for the
existing 90-day restricted token. No redirect URI, state, API key, password,
or browser credential is exposed in conversation.

The returned manifest is client-aware. Hermes receives a direct Markdown Skill
URL rather than the MyAgents ZIP installer URL. The Hermes onboarding Skill
uses Hermes' own configuration surface to add only origin-derived entries:

- custom OpenAI Chat Completions provider `the-one-bolierxiang-cn`, with
  `https://the-one.bolierxiang.cn/v1`, the approved model, and
  `key_env: THE_ONE_API_KEY`;
- `THE_ONE_API_KEY` only in `~/.hermes/.env`;
- remote HTTP MCP `the-one-gateway-bolierxiang-cn` at `/mcp`, with its
  authorization header referring to the environment secret;
- post-connection `the-one-gateway` Skill from the official Markdown URL.

The Skill must preserve `model.default` and all unrelated Hermes providers,
MCP entries, Skills, and secrets. It checks the pre-existing default model
before writing and verifies it remains unchanged afterward.

## Safety requirements

- The user, not Hermes, signs in and completes 2FA or final confirmation.
- Hermes may use only `https://the-one.bolierxiang.cn` and Hermes' documented
  local configuration surfaces.
- It never downloads or executes a binary, installer, source clone, shell
  bootstrap, or arbitrary Skill archive.
- It never reads, prints, logs, stores in Skill text, or asks the user to paste
  passwords, session data, PKCE values, or API keys.
- It stops without changing user configuration if Hermes does not expose the
  native provider, secret, MCP, and Skill operations needed for this flow.

## Public routes

- `/skills/hermes/SKILL.md`: onboarding/pairing Skill.
- `/skills/hermes/the-one-gateway/SKILL.md`: post-connection usage Skill.

Both are build-time embedded, serve `text/markdown`, and share the existing
version header. The existing MyAgents paths and archive remain unchanged.

## Tests and acceptance

- Model tests accept `hermes-skill` pairing and reject malformed pairing
  requests exactly as MyAgents pairing does.
- Controller tests ensure the Hermes manifest returns the Hermes usage Skill
  URL while MyAgents continues returning its ZIP source.
- Router tests serve both Hermes documents and assert the onboarding document
  excludes executable bootstrap and credential disclosure instructions.
- Acceptance is the one user instruction above: Hermes asks only for browser
  confirmation, configures The One, verifies provider and MCP connectivity,
  and leaves the current default model unchanged.
