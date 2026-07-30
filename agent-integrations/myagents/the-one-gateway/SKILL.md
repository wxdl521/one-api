---
name: the-one-gateway
description: Use The One through an already configured MyAgents provider and its read-only MCP connection tools. Use when checking The One connection status, available models, usage, or reconnecting an expired MyAgents integration.
---

# The One Gateway

Use the existing **The One** model provider configured in MyAgents. Keep the
user's current default provider unchanged and select The One only when the task
calls for it.

## Check the connection

Use the configured The One MCP server for read-only account information:

- `the_one_connection_status` for connection and credential-expiry status.
- `the_one_list_models` for the model pinned to this connection.
- `the_one_usage` for account usage information.
- `the_one_reconnect` for the safe reconnect command.

Treat the MCP results as read-only diagnostics. Do not use the MCP server to
create, rotate, revoke, inspect, or disclose credentials.

## Use the configured model

Use the The One provider and its configured model through MyAgents' normal
OpenAI-compatible chat-completions integration. The provider credential is
managed by MyAgents; never ask the user to paste that credential into a prompt,
tool call, browser form, or chat message.

## Reconnect

When the connection is expired, revoked, or unavailable, read and follow the
official pairing Skill at
`https://the-one.bolierxiang.cn/skills/myagents/SKILL.md`. It opens the
official authorization page; wait for the user to log in, select a group and
model, and confirm access.

The browser flow must remain user-operated. Never read, print, copy, store,
auto-fill, submit, or request the user's password, two-factor code, session
cookie, or API key. Never expose a credential in command output, logs, tool
arguments, screenshots, or error reports.

## Version

`1.1.0`
