---
name: the-one-myagents-pairing
description: Use when a user asks MyAgents to safely connect The One from the official pairing page without changing their default provider.
---

# Safely connect The One to MyAgents

Use this Skill only for the user's The One connection. The official gateway is
`https://the-one.bolierxiang.cn`; do not substitute another origin.

## Non-negotiable safety rules

- Never download or run executables, source clones, shell bootstrap scripts, or
  external installers for this connection.
- Never inspect, copy, print, log, or disclose passwords, two-factor codes,
  session cookies, browser storage, API keys, PKCE values, or secret headers.
- The user alone signs in, enters two-factor codes, selects the group/model,
  and clicks the final confirmation in their browser. Do not automate or fill
  in those actions.
- Use only the official origin and MyAgents' native local configuration
  surface. Do not alter unrelated providers, MCP servers, Skills,
  credentials, account settings, or any user data.
- Do not change the current default provider. If MyAgents does not expose a
  supported native provider, HTTP MCP, and user Skill configuration operation,
  stop and explain that the connection cannot be completed safely.

## Pair the account

1. Generate a cryptographically random PKCE verifier in memory. Derive an S256
   URL-safe base64 challenge from it. Do not display either value.
2. Send a JSON request to
   `https://the-one.bolierxiang.cn/api/agent-connect/pairings` with only
   `code_challenge` and `code_challenge_method: "S256"`.
3. Confirm that the response has an `authorization_path` beginning exactly with
   `/agent-connect?request_id=`. Open that path only after prefixing it with
   `https://the-one.bolierxiang.cn`.
4. Tell the user that the official page is open and wait. Do not request
   credentials, authentication codes, or an API key in chat.
5. Poll exactly the returned `exchange_path` with `POST` at the returned
   `poll_interval_seconds`, and stop when `expires_at` is reached. Send JSON
   `{ "code_verifier": "<the in-memory verifier>" }` to
   `https://the-one.bolierxiang.cn` plus that path. A successful pending result
   is `{ "success": true, "data": { "pending": true } }`; wait and retry.
   A successful completed result contains the provider manifest in `data`.
   Do not probe alternate paths or field names, and do not emit the request ID,
   verifier, or response body in conversation or diagnostics.
6. After a successful exchange, retain the returned API key only in MyAgents'
   secret configuration fields. Do not display it, store it in a Skill file,
   include it in a command, or use it outside the provider and MCP entries
   below.

## Configure only The One entries

Use MyAgents' native local configuration capability, or its documented local
management API, to create or update idempotently scoped entries derived from
the official origin. Keep IDs stable across retries and preserve every current
default selection.

- Provider: name `The One`; OpenAI Chat Completions protocol; base URL
  `https://the-one.bolierxiang.cn/v1`; only the approved `model`; store the
  returned API key in its secret field.
- MCP: name `The One Gateway`; HTTP URL
  `https://the-one.bolierxiang.cn/mcp`; configure the returned API key only in
  its secret Bearer credential field; enable it without changing global or
  default provider settings.
- Skill: install the user-scoped `the-one-gateway` usage Skill from
  `https://the-one.bolierxiang.cn/skills/myagents/the-one-gateway.zip`. This
  archive contains only the versioned Markdown Skill; let MyAgents' native
  Skill installer handle it and never execute archive contents manually.

After configuration, use the provider's normal model listing and the MCP
connection test. Report only that the provider, read-only MCP, and usage Skill
were configured, or which non-secret step failed. If a partial write fails,
retry the same origin-derived entries; never create duplicates or replace an
unrelated entry.

## Reconnect

When the MCP reports an expired, revoked, or unavailable connection, repeat
this Skill's pairing procedure. The user must again complete the official
browser confirmation.
