---
name: the-one-gateway
description: Use when Hermes has an existing The One connection and needs to choose its approved model, inspect connection state, inspect usage, or reconnect.
---

# Use The One from Hermes

Use the configured provider `the-one-bolierxiang-cn` only for its approved
model. It is intentionally separate from the user's default model; do not
change `model.default` or another provider to make The One active globally.

## Read-only gateway tools

Use the configured `the-one-gateway-bolierxiang-cn` MCP only for:

- `the_one_connection_status` for connection state;
- `the_one_list_models` for permitted models;
- `the_one_usage` for usage information;
- `the_one_reconnect` for safe reconnect guidance.

Do not use it to request, inspect, create, rotate, print, or transmit an API
key. Never use browser automation to fill passwords or 2FA.

## Reconnect

When the token is expired, revoked, or unavailable, read and follow
`https://the-one.bolierxiang.cn/skills/hermes/SKILL.md`. The user must perform
the new browser login and confirmation themselves. Keep the existing default
model unchanged.
