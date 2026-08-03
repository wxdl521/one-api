# PromptShot gateway compatibility design

## Goal

Make The One the production gateway for the existing PromptShot desktop client. The client keeps its locked request contract and holds only its existing one-token. The One performs authentication, selects a compatible model from the token's usable group and optional model allow-list, relays the request through its normal channel, quota, retry, and logging path, and returns PromptShot-shaped responses.

PromptShot's production base URL changes from `https://sg-admin.boliboliworld.cn` to `https://the-one.bolierxiang.cn`. This is a client-only host change; development overrides remain available.

## Considered approaches

1. Change PromptShot to use public OpenAI/Gemini APIs directly. This would break its locked four-endpoint contract, require client-side model selection, and make the client responsible for provider protocol details. It is rejected.
2. Keep the separate Node PromptShot proxy and make it call a private The One endpoint. This duplicates authentication, rate limiting, error handling, and deployment while the target gateway already owns those responsibilities. It is rejected.
3. Add a native PromptShot compatibility adapter to The One. The adapter turns the four PromptShot request shapes into internal standard relay requests, lets the existing relay choose channels and charge quota, then turns successful relay responses back into the PromptShot schema. It is selected.

## API and authentication

The One will serve these existing endpoints without changing the desktop client contract:

- `POST /v1/auth/validate`
- `POST /v1/reverse-prompt`
- `POST /v1/generate-image`
- `POST /v1/clean-image`

`/v1/auth/validate` accepts its token only in the JSON body as PromptShot already does. A bounded request parser restores the body after extracting the token, supplies it to the same token-authentication path used by relay requests, and emits only the permitted account/plan/quota metadata. It must not log the token or echo it in any response.

The other endpoints continue to require `Authorization: Bearer <one-token>` and pass through The One's normal token-status, expiry, IP restriction, user-status, quota, and model-limit enforcement. They are protected by the same system-performance, request-body, model-rate-limit, and channel-distribution controls as normal `/v1` relays.

## Model selection

The One gains one server-side, admin-configurable PromptShot model-policy option with ordered candidates for `reverse`, `generate`, and `clean`. Each candidate declares the logical model and the standard relay operation it supports. Model names remain server configuration; PromptShot never sends, stores, or displays them.

For a request, the adapter chooses the first candidate that:

1. is allowed by the one-token's model limit when that limit is enabled;
2. is enabled in the token's effective group (or an eligible auto-group); and
3. has a channel that supports the candidate's relay operation.

No candidate is inferred from model-name prefixes. This prevents a text-only model from being selected for image generation merely because it happened to be enabled in the group. If no candidate qualifies, the adapter returns a safe `403` explaining that the token's group has no compatible PromptShot model; it never falls back to a wider group or a model outside the token allow-list.

The initial candidate policy is intentionally empty until an administrator configures actual models available in this deployment. This avoids embedding unverified model IDs such as historical PromptShot defaults into source code.

## Relay translation and response shaping

The adapter runs as an internal request/response middleware pair:

- Reverse-prompt requests become a non-streaming multimodal chat-completions request containing the source image and a server-owned JSON-only instruction. The adapter validates the returned JSON and returns the required Chinese prompt, English prompt, and seven structured fields.
- Generate-image requests become an image-generation request when no reference image is present, or the selected candidate's image-edit operation when a reference is supplied.
- Clean-image requests become an image-edit request with the server-owned cleanup instruction and the input image.

Before the relay executes, the translated body has a selected model and standard request path, so existing middleware performs distribution, quota pre-consumption, retries, settlement, and consumption logs exactly once. A buffered non-streaming response adapter converts success payloads to PromptShot's `image_b64`/`mime` or reverse-prompt schema. Image output must be supplied as base64 by the selected channel or safely materialized under an explicit bounded server-side image-download policy; URLs are never returned to the client because the PromptShot contract does not accept them.

The adapter maps errors to PromptShot's existing contract: malformed input is `400`, invalid/disabled/expired authorization is `401` or `403`, insufficient quota is `402`, rate limiting is `429`, and channel/upstream faults are generic `5xx` messages. It never exposes upstream response bodies, provider credentials, internal model names, stack traces, or database details.

## PromptShot host migration

PromptShot source changes only its production default and its allow-list/CSP to `https://the-one.bolierxiang.cn`:

- Rust `DEFAULT_PROXY_BASE` and host-validation tests;
- Svelte default proxy state and development fallback;
- Tauri `connect-src` CSP;
- user-facing production URL documentation and probe defaults.

Local `PROMPT_SHOT_PROXY_BASE_URL` and `VITE_DEV_PROXY_BASE` overrides are preserved for development and mock testing.

## Verification

Backend tests will use deterministic Gin contexts and a configured SQLite fixture to prove:

- the body-token validation route uses the same token validity rules and never returns the token;
- selection honors group membership, model limits, candidate order, and rejects no-match cases without cross-group fallback;
- each PromptShot payload translates to the intended standard relay shape and preserves its image/base64 bounds;
- standard relay failures map to the required PromptShot status/message contract; and
- a successful normalized response has every required PromptShot field.

PromptShot tests will prove every production source default and CSP points to The One, while local overrides continue to work. The relevant Go test package, PromptShot `pnpm check`, PromptShot security gate, and focused Rust tests will be run before handoff.

## Scope boundaries

This change does not alter existing default models, existing provider entries, agent-connect configuration, general relay endpoints, or unrelated PromptShot UI behavior. It does not add client-side API-key handling or accept model selection from the desktop client.
