# WeChat Mini Program Privacy Inventory and Release Checklist

This inventory describes the phase-one Mini Program implementation. It is an engineering record, not a substitute for the organisation's privacy policy, legal review, or the current WeChat platform requirements. Re-review it with every new permission, client storage key, BFF field, upstream recipient, analytics SDK, or telemetry event.

## Privacy inventory

| Data | Purpose and flow | Storage and retention | Disclosure and logging boundary |
| --- | --- | --- | --- |
| WeChat login code | The client calls `Taro.login` (the WeChat `wx.login` API) to obtain a one-time code, then sends it to the Mini Program BFF. Only the BFF calls WeChat's `code2Session` endpoint. | Request memory only; it is not persisted. | Sent from the client to the configured BFF, then from the BFF to WeChat. Do not log the code. |
| OpenID and WeChat `session_key` | WeChat returns these to the BFF while resolving the Mini Program subject. The BFF uses the OpenID to derive the subject identity. | Plaintext OpenID and `session_key` are cleared from the exchange result and are not returned or persisted. The durable identity and binding records retain an HMAC-derived OpenID digest plus AppID. | Do not return, log, or expose either plaintext value. The AppSecret remains in the protected server secret store. |
| Pending identity ticket, binding ticket, and binding ID | Opaque credentials coordinate account registration and browser binding. The binding URL carries its one-time ticket in the fragment. | Pending and browser binding flows expire after five minutes. Expired or consumed auth-flow and binding records are removed by the hourly cleanup after the 24-hour cleanup retention. The client keeps the pending ticket in memory only. | Never put tickets in query parameters, logs, analytics, crash reports, or screenshots. The browser binding page removes the fragment from the visible URL. |
| Mini Program access token and session ID | Authenticate BFF calls and allow session renewal with a new WeChat login code. | The Mini Program keeps them in process memory only; it does not write them to Taro/WeChat storage. Server sessions follow the existing session retention and revocation policy. | Send only in the HTTPS `Authorization` header. Do not log headers, token values, or session credentials. |
| Account, plan, product, order, and Mini Program token metadata | Show the signed-in user's own account and limited token-management views. | The client does not persist these responses. Server records use their existing ownership, access-control, and retention rules. | BFF responses are user-scoped. A full Mini Program token key is returned once at creation; do not log it or retain it in client storage. Clipboard use is user initiated. |
| Text-test prompt, selected model, and upstream output | A signed-in user can make a restricted text test only when the feature flag and server allowlist permit it. The BFF sends the prompt through the existing Relay to the selected authorised upstream. | Prompt and output are transient request data. The Mini Program does not persist them. The text-attempt record retains the user ID, client request ID, model, input HMAC, server-generated claim nonce for idempotency, state, charge reference, charged quota, approved terminal error code, and lifecycle metadata (created, started, completed, and expiry times) for 24 hours before scheduled cleanup. Existing Relay and consume-log retention rules still apply. | Never store prompt text or model output in Mini Program attempt records, application logs, new analytics, or client storage. Do not expose channel, upstream, credential, or raw provider-error details to the client. |
| Pending text-test request ID | Lets the client recover the safe terminal status after a foreground/background interruption. | The client stores only a validated UUID in `miniapp.pending-text-test-request-id.v1`; it is removed on completion or session clear. | It is not a credential and must not be paired with prompt or output in telemetry. |
| IP address and user agent | Existing server-session and rate-limit protections use request metadata. | Retained under the existing session/log policies, not by a new Mini Program store. | Follow the existing access-log privacy controls; do not add raw request bodies or credentials to these records. |

The Mini Program does not request profile, phone-number, location, contacts, camera, microphone, photo-album, or user-info permissions. It does not include a client analytics SDK or call a third-party API directly. Its application API client targets only the configured HTTPS BFF; browser handoffs use only the verified console origin. The client uses `Taro.login` to obtain a WeChat code, while the BFF performs `code2Session` and authorised model-provider calls server-side.

## Public build configuration and server secrets

`miniapp/.env.example` contains the only client-build settings:

- `TARO_APP_API_BASE_URL` is the public HTTPS gateway origin.
- `TARO_APP_MINIAPP_BINDING_ORIGIN` is the public HTTPS console origin used only to validate the exact `/miniapp-bind` handoff page.

Both values are compiled into the Mini Program and must be treated as public. Copy the example to an ignored local `.env`, replace the `.invalid` placeholders for the intended environment, and keep development, test, staging, and production origins separate. The client fails closed when either value is absent or malformed.

Configure the following only in the server deployment and protected secret store:

- `WECHAT_MINIAPP_APP_ID` is public but must be isolated per environment.
- `WECHAT_MINIAPP_APP_SECRET` and `WECHAT_MINIAPP_SUBJECT_HMAC_KEY` are server-only secrets. The HMAC key must be high-entropy, stable across all nodes, and rotated only through an explicit identity migration.
- `MINIAPP_BIND_WEB_BASE_URL` must be the exact `/miniapp-bind` route on the approved origin.
- `MINIAPP_ALLOWED_MODELS` is the server-side exact model allowlist. An empty value allows no Mini Program models.
- `MINIAPP_HTTP_TIMEOUT_SECONDS` is the server-side WeChat request timeout and defaults to 10 seconds.

Never put AppSecret, the subject HMAC key, `session_key`, access tokens, opaque tickets, server URLs containing credentials, or upstream credentials in `miniapp/`, `project.config.json`, CI variables printed to logs, or Mini Program build artifacts.

## Developer verification checklist

- [ ] Start from the `miniapp/` directory with a supported Bun version and a clean lockfile.
- [ ] Run `bun install --frozen-lockfile`.
- [ ] Run `bun run typecheck`, `bun run test`, and `bun run build:weapp`.
- [ ] Review the generated `miniapp/dist/` output before handing it to WeChat Developer Tools. It must not contain any real server-only secret, credential, opaque-ticket, prompt, or model-output value from a test fixture. Static protocol field names alone are not sensitive values.
- [ ] Verify the build uses the intended public API and binding origins, with no `.invalid` placeholders.
- [ ] Import `miniapp/` (not `web/`) into WeChat Developer Tools. The tracked `project.config.json` deliberately uses `touristappid`; never commit a real environment AppID into client source.
- [ ] Exercise first login, existing-account binding, binding expiry, logout, session renewal, token one-time display/revocation, checkout handoff, text-test status recovery, and disabled-feature responses.
- [ ] Confirm that an invalid request domain, invalid binding origin, expired ticket, revoked session, and backgrounded text test fail safely without replaying a request or exposing sensitive data.

The `Mini Program` GitHub Actions workflow runs the locked Bun install, typecheck, unit tests, and WeChat build for Mini Program changes. It intentionally uses `pull_request`, rather than `pull_request_target`, before it executes repository code.

## WeChat platform and release checklist

- [ ] Confirm the Mini Program subject, administrators, developers, and operators in the WeChat console; select the AppID for this environment in Developer Tools or the protected release process.
- [ ] Confirm the chosen service category, publication/filing status, required qualifications, and current review materials with operations and legal. Recheck the current WeChat console rules rather than relying on this document.
- [ ] Register and verify the production BFF HTTPS origin as the request legal domain. Test its DNS, certificate chain, TLS settings, redirects, and real-device reachability; do not ship with Developer Tools domain checking disabled.
- [ ] Register and verify the approved console origin as the business domain before enabling the browser binding or checkout handoff. Validate that the exact `/miniapp-bind` and `/miniapp-checkout` flows work in the real Mini Program subject and supported base library.
- [ ] Reconcile the WeChat privacy-protection guide, privacy policy, user agreement, customer-support/contact path, complaint path, and third-party personal-information disclosure with the inventory above. Include the server-side WeChat identity exchange and authorised model-provider processing where applicable.
- [ ] Build an experience version with only environment-isolated public values. Inspect the package size and subpackage boundary in WeChat Developer Tools, then preview it on current iOS and Android devices over real networks.
- [ ] Verify no WeChat permission prompt is triggered in phase one. If a future feature needs a permission or adds a data field, update this inventory and the WeChat privacy materials before merging it.
- [ ] Keep `MiniProgramEnabled` and `MiniProgramTextTestEnabled` disabled until deployment configuration, domain verification, privacy/compliance review, model allowlist, rate-limit policy, upstream authorisation, and the real-device checks are complete. Enable the flags gradually and independently.
- [ ] Capture the release version, AppID environment, approved origins, feature-flag decision, reviewer, test devices, and rollback owner in the release record. Do not include credentials or personal data.

## Operational safety and rollback

Use the existing Mini Program route tag, request correlation ID, session audit records, rate-limit outcomes, and consume-log request reference for operational diagnosis. Safe operational fields are route category, outcome class, HTTP status, elapsed time, request correlation ID, feature-flag state, and aggregate count. For a text test, administrators can correlate its request ID with the existing consume log without logging its prompt or output.

Never add or retain `Authorization` headers, access tokens, `wx.login` codes, OpenIDs, `session_key`, AppSecret, subject HMAC key, opaque tickets, raw request bodies, prompts, model outputs, provider response bodies, or user names in logs, metrics labels, traces, dashboards, alerts, or incident tickets. This hardening intentionally adds no new logging or metrics hook.

If a release is unsafe, disable `MiniProgramTextTestEnabled` first to stop the text-test capability, then disable `MiniProgramEnabled` to close the BFF. Revoke affected sessions or Mini Program tokens through the existing server controls, preserve only the existing safe correlation records needed for incident response, fix configuration or code, and repeat the full developer and real-device checklist before re-enabling either flag.
