---
name: the-one-hermes-pairing
description: Use when a user asks Hermes to connect The One from its official URL while retaining the current default model.
---

# Safely connect The One to Hermes

Use this Skill only to connect the user to `https://the-one.bolierxiang.cn`.
The user should only need to operate the official browser page.

## Safety boundaries

- Do not change the current default model, `model.default`, or any existing
  provider selection. Record it before configuration and verify it is exactly
  unchanged afterward.
- Do not modify unrelated providers, MCP servers, Skills, secrets, account
  settings, or user files.
- Never ask for, read, copy, print, log, or disclose passwords, 2FA codes,
  browser storage, session cookies, PKCE values, or API keys.
- The user alone signs in, completes 2FA, chooses group/model, and confirms the
  request in the official browser page. Do not automate those browser actions.
- Never open `authorization_url` in an agent-controlled browser, CDP session,
  headless Chromium, Browserbase, or browser automation tool. Send it only to
  the user's own system browser. If the user is chatting in WeChat or another
  in-app WebView, tell them to use the menu to open the link in their system
  browser instead.
- Do not download or execute binaries, installers, source clones, bootstrap
  scripts, or archives. Use only Hermes-native provider, secret, MCP, and Skill
  configuration operations.
- Stop and explain the limitation if those Hermes-native operations are not
  available. Do not substitute a manual key-paste flow.

## Pair the user account

1. Keep a cryptographically random PKCE verifier only in current-process
   memory. Derive its S256 URL-safe challenge without displaying either value.
2. Create the pairing with a JSON POST to
   `https://the-one.bolierxiang.cn/api/agent-connect/pairings`:

   ```json
   {"client_kind":"hermes-skill","code_challenge":"<derived S256 value>","code_challenge_method":"S256"}
   ```

3. Require an absolute `authorization_url` on the official The One origin.
   Send that URL privately to the user with this instruction: “Please open it
   in your system browser, sign in, choose a group and model, then confirm.
   Do not use an in-app browser.” The URL may be shown to that user, but never
   copy its request ID into logs, Skills, repositories, or public chat.
4. Poll only the returned `exchange_path`, ending at `expires_at`. POST the
   in-memory verifier as `code_verifier`. Treat either `data.pending == true`
   or a legacy top-level `pending == true` as waiting. Sleep for
   `data.poll_interval_ms` (at least 3000ms) before each retry. On HTTP 429,
   honor `Retry-After` and use exponential backoff capped at 30 seconds.
   Stop immediately on `denied`, `expired`, `revoked`, or `invalid_verifier`.
   A completed response has the manifest in `data.manifest`. Never reveal the
   verifier, API key, cookie, or raw exchange response body.
5. Retain the returned API key only in Hermes secret storage as
   `THE_ONE_API_KEY` in `~/.hermes/.env`. Never place the key in a Skill,
   command line, visible configuration value, conversation, or diagnostic.

## Configure only stable The One entries

Use Hermes' native configuration capability to create or update these exact
origin-derived entries idempotently. Merge only the named entries below;
retain all other configuration.

```yaml
custom_providers:
  - name: the-one-bolierxiang-cn
    base_url: https://the-one.bolierxiang.cn/v1
    key_env: THE_ONE_API_KEY
    api_mode: chat_completions
    model: <manifest model>
mcp_servers:
  the-one-gateway-bolierxiang-cn:
    url: https://the-one.bolierxiang.cn/mcp
    headers:
      Authorization: "Bearer ${THE_ONE_API_KEY}"
    enabled: true
    tools:
      include:
        - the_one_connection_status
        - the_one_list_models
        - the_one_usage
        - the_one_reconnect
```

Install the post-connection Skill with Hermes' native Skill installer from:

`https://the-one.bolierxiang.cn/skills/hermes/the-one-gateway/SKILL.md`

Before configuration, snapshot the exact values of `model.default` and
`model.provider`. Do not set either field or any global model setting. After
configuration, verify both values are byte-identical to the snapshot, the named
provider lists the approved model, and the named MCP connects. Report only
success or the non-secret failed step. On partial failure, revert only this
attempt's named provider/MCP/Skill entries and never remove or overwrite
unrelated configuration.

## Cancellation and cleanup

If the user says stop, cancels, or does not complete the request before expiry,
immediately stop polling and delete the in-memory verifier and temporary
metadata. Do not write a key or leave half-configured named entries. Never
restart pairing automatically; wait for an explicit new request from the user.

## Reconnect

If the MCP reports that the connection is expired, revoked, or unavailable,
repeat this Skill. The user must complete a new browser confirmation.
