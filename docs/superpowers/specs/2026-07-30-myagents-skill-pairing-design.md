# MyAgents Skill Pairing Design

## Goal

Let a MyAgents agent connect a user to The One after reading a stable Skill URL.
The user only logs in, selects an eligible group and model, and confirms in the
official browser page. The agent performs the local MyAgents configuration
without downloading or executing an external binary.

## User-facing contract

The only instruction the user needs to send is:

```text
Read https://the-one.bolierxiang.cn/skills/myagents/SKILL.md and safely connect The One. Keep my current default provider unchanged.
```

The public document is versioned in the repository and served from the same
origin as the gateway. It is the onboarding Skill; the existing
`the-one-gateway` Skill remains the post-connection usage Skill.

## Chosen flow

```text
MyAgents Agent                    The One                         User
     | create pairing session        |                               |
     |------------------------------>|                               |
     | request id + authorization URL|                               |
     |<------------------------------|                               |
     | open official URL                                             |
     |-------------------------------------------------------------->|
     |                               |<--- login, choose, confirm ---|
     | poll with PKCE verifier       |                               |
     |------------------------------>|                               |
     | one-time restricted manifest  |                               |
     |<------------------------------|                               |
     | configure native provider, MCP, and usage Skill               |
```

### Pairing endpoints

`POST /api/agent-connect/pairings` creates a short-lived session. It accepts
only `client_kind: "myagents-skill"`, an S256 PKCE challenge, and method
`S256`; it does not accept a redirect URI. The result includes a request ID,
an official authorization URL, and a poll interval. The opaque request ID and
the PKCE verifier are held only in the agent's memory.

The existing signed-in request page remains the authorization surface. For a
pairing request, confirmation completes the request in place and renders a
success page rather than redirecting to loopback.

`POST /api/agent-connect/pairings/:request_id/exchange` accepts the original
PKCE verifier. Before browser confirmation it returns an explicit pending
state and no token. After confirmation it atomically consumes the request,
creates the existing 90-day model- and group-restricted token, and returns the
same manifest shape used by the local connector. Expired, cancelled, already
consumed, or invalid-verifier requests return no token.

Pairing creation and polling are rate-limited. Request IDs, authorization
codes, verifiers, API keys, and browser credentials are never placed in logs,
page text, diagnostics, or Skill output.

### MyAgents configuration

The onboarding Skill uses MyAgents' own local configuration capability (native
tool or documented local admin API). It creates or updates only stable
origin-derived IDs and does not set a default provider:

- an OpenAI Chat Completions provider using `<origin>/v1`, the approved model,
  and the restricted token;
- the read-only HTTP MCP at `<origin>/mcp` with its Bearer header;
- the user-scoped `the-one-gateway` usage Skill.

It verifies the model list and MCP connection, reports only success/failure,
and retains the token exclusively in MyAgents' secret configuration. Partial
configuration is retried idempotently.

## Skill safety rules

The public onboarding Skill must explicitly require all of the following:

- no executable downloads, source cloning, shell bootstrap scripts, or external
  installer execution;
- no password, two-factor code, session cookie, browser-storage, or API-key
  inspection, copying, printing, logging, or chat disclosure;
- browser login and final authorization are user-operated;
- only `https://the-one.bolierxiang.cn` and MyAgents' local configuration
  surface are valid targets;
- abort if MyAgents does not expose a supported native configuration operation;
- preserve the current default provider and never modify unrelated providers,
  MCP servers, Skills, credentials, or account settings.

## Public document serving

The gateway serves these Markdown documents with `text/markdown` and a stable
version header:

- `/skills/myagents/SKILL.md` — onboarding/pairing Skill;
- `/skills/myagents/the-one-gateway/SKILL.md` — post-connection usage Skill.

The service embeds the repository documents at build time, so the public URL
and the tagged source describe the same behavior.

## Data model and compatibility

Pairing sessions extend the existing `agent_connect_requests` table rather than
introducing a second authorization subsystem. A pairing-mode boolean is added
without a database default tag, using GORM-compatible migration behavior for
SQLite, MySQL 5.7.8+, and PostgreSQL 9.6+. Existing loopback PKCE requests and the local
`the-one-connect` CLI remain compatible.

## Tests and acceptance

- model and controller tests cover pairing creation validation, S256 verifier
  validation, pending polling, authorization, expiry, cancellation, concurrent
  exchange, and exactly-once token issuance;
- router tests cover public Markdown response headers and no credential content;
- Skill tests exercise an agent pressure scenario: it must use native MyAgents
  configuration, wait for user browser confirmation, and refuse executable
  downloads or credential disclosure;
- frontend tests cover the pairing completion screen and ordinary loopback
  callback behavior;
- manual acceptance: a MyAgents agent receives the single instruction above,
  opens the official page, waits for the user's confirmation, configures the
  provider/MCP/usage Skill, and leaves the prior default provider unchanged.

## Implementation verification

- Implemented on `codex/harden-epay-topups` at `aef8431dc` without changing
  GitHub `main` (`3be1e1734`).
- Local verification passed at 2026-07-30T09:47:15Z:
  `go test ./model ./service ./controller ./router ./middleware ./cmd/the-one-connect -count=1`,
  `bun run typecheck`, and `bun run build` from `web/`.
- Deployed image `the-one:next-aef8431d` at 2026-07-30T09:47:15Z. The prior
  image is retained as stopped container `the-one-rollback-1bff0b58` for
  immediate rollback.
- Public verification passed for `https://the-one.bolierxiang.cn/` and
  `https://the-one.bolierxiang.cn/skills/myagents/SKILL.md`; the Skill response
  is `text/markdown; charset=utf-8` with `X-The-One-Skill-Version: 2026-07-30`.
