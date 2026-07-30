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

3. Require an `authorization_path` beginning exactly with
   `/agent-connect?request_id=`. Open only that path prefixed by
   `https://the-one.bolierxiang.cn`, tell the user it is ready, and wait for
   their confirmation.
4. Poll only the returned `exchange_path` at its returned interval, ending at
   `expires_at`. POST the in-memory verifier as `code_verifier`. A response
   with `pending: true` means wait; a completed response has the manifest in
   `data`. Never reveal the request ID, verifier, or response body.
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

Do not set `model.default`, `model.provider`, or any global model setting.
Verify that the original default model is unchanged, the named provider lists
the approved model, and the named MCP connects. Report only success or the
non-secret failed step. On partial failure, retry only these named entries and
never remove or overwrite unrelated configuration.

## Reconnect

If the MCP reports that the connection is expired, revoked, or unavailable,
repeat this Skill. The user must complete a new browser confirmation.
