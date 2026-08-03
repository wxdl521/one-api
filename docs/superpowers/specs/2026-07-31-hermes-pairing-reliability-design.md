# Hermes pairing reliability design

## Scope

Implement the non-device-code portions of the 2026-07-31 pairing improvement
proposal. The user still performs login, two-factor authentication, selection,
and confirmation in their own system browser. Hermes never opens the
authorization URL in an agent-controlled browser and never changes the default
model or provider.

## Pairing contract

`POST /api/agent-connect/pairings` keeps the existing fields for compatibility
and additionally returns an absolute `authorization_url`, a three-second
`poll_interval_ms`, `expires_in`, and the existing `expires_at`. The relative
paths remain canonical API routes.

The pairing exchange uses one stable shape while waiting:

```json
{"pending":true,"status":"waiting_user","poll_interval_ms":3000,"expires_in":420}
```

Completed exchanges retain their legacy fields and additionally expose a
`manifest` containing the same API key, selected model/group, provider/MCP
metadata, and skill source. The key remains available only in the successful
exchange response.

## Failure, rate limit, and UX

Controller errors acquire stable machine-readable codes without removing the
current human-readable message. Pairing exchange requests use a dedicated
per-request throttle. A client exceeding it receives 429 plus `Retry-After`;
the public Skill honors that header and backs off rather than tight-looping.

The authorization page shows a non-blocking in-app-browser warning and an
unambiguous completion message. It does not expose secrets or alter the
fresh-login binding already deployed.

## Hermes Skill behavior

The public pairing Skill explicitly sends the absolute authorization URL to the
human and prohibits `browser_navigate`, CDP, Browserbase, or any controlled
browser from opening it. It recognizes both top-level and nested pending state,
honors interval and retry headers, removes the in-memory verifier on every
terminal path, and snapshots/asserts default model/provider before and after
the named-entry configuration merge.

## Compatibility and tests

Existing MyAgents/Hermes callers using the legacy paths and fields continue to
work. Tests cover response compatibility, nested pending values, manifest
shape, throttle response headers, stable error codes, and no default-model
mutation. No device-code endpoint or user-code field is introduced in this
change.
