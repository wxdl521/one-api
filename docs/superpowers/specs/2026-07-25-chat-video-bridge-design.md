# Chat-client Seedance bridge design

## Goal

Let users of chat-only OpenAI-compatible clients, beginning with Cherry Studio and Open WebUI, select an eligible Seedance model and generate a video without configuring a separate video endpoint. The One will translate the chat request into its existing asynchronous video-task API, then return either the finished video or a secure, auto-refreshing task page.

## Scope

The first release supports text-to-video only for official Seedance task models already supported by the Doubao video adaptor. It deliberately does not make every video model appear as a chat model, does not alter third-party clients, and does not change the existing `/v1/video/generations` contract.

Supported first-release model IDs are the `relay/channel/task/doubao.ModelList` values, such as `doubao-seedance-2-0-260128` and `doubao-seedance-2-0-fast-260128`. A model must also resolve to a configured, enabled task channel available to the caller's group.

## User experience

1. A user configures only The One base URL and an API token in Cherry Studio or Open WebUI.
2. The user selects a Seedance model and submits a normal text chat message.
3. The One accepts `POST /v1/chat/completions`, detects an eligible Seedance model, and submits the equivalent internal video task.
4. The bridge waits for a configurable window, defaulting to 5 minutes.
5. If the task completes in that window, the response is an OpenAI chat completion whose assistant content is a Markdown video link. The link plays or downloads through The One's authorized video proxy.
6. If the task is still running at the deadline, the response is an OpenAI chat completion whose assistant content contains a secure task-page link. The page polls its task-status endpoint and changes to an embedded player/download control after completion.
7. If the upstream task fails while waiting, the bridge returns a normal OpenAI-style error. If it fails after the task page has been returned, the page renders the sanitized failure message.

For `stream: true`, the bridge holds the stream during the same wait window and emits one final OpenAI-compatible SSE content chunk plus `[DONE]`. On timeout it emits the task-page link chunk. It does not fake token-by-token video generation.

## Architecture

### Chat-video eligibility and configuration

Add a server-level `ChatVideoBridge` setting group:

- `Enabled` (default `false`) keeps the feature opt-in.
- `MaxWaitSeconds` (default `300`, bounded to `0..600`) controls the synchronous wait window.
- `TaskPageTTLSeconds` (default `86400`, bounded to `300..604800`) controls task-page access tickets.
- `Models` is an explicit allow-list. Defaults are empty; admins must opt in to the exact configured Seedance model IDs they want exposed as chat models.

An eligibility check must require all of the following: bridge enabled, model in the allow-list, model supported by the Seedance task adaptor, and an accessible task channel. This prevents a misspelled or unrelated chat model from being silently treated as a video request.

### Shared task submission path

Refactor the currently controller-owned task submission loop into an exported internal function usable by both `RelayTask` and the bridge. It must preserve the existing task relay sequence: authentication, model-to-channel distribution, request validation, bounded billing/pre-consume, retry behavior, task persistence, task polling, and settlement.

The bridge converts the final user text message into `relaycommon.TaskSubmitReq` with the selected model and calls that shared task path using task relay format. It rejects chat tools, `response_format`, audio, file/image parts, and empty final-user content with a clear `400` in the first release. Earlier system/developer messages may be retained only as a bounded textual prompt prefix; the first version should instead use the final user message alone to avoid ambiguous instruction composition.

### Completion wait and response formatting

After submission, the bridge reads the persisted task record rather than polling the upstream provider itself. It uses the existing task polling service as the source of truth and checks the local task state at a small bounded interval until the deadline. This avoids duplicate upstream polling and keeps billing/refund logic in one place.

Completed tasks produce a valid OpenAI `chat.completion` response. The message content includes a concise success message and a Markdown link to the signed The One content route. Timed-out tasks produce a valid `chat.completion` response with a task-page link. Existing text-model relay behavior remains unchanged.

### Secure public task page

Add a public, ticket-authorized task page under the web application, for example `/chat-video/tasks/:taskID?ticket=...`. It only shows the task state, progress, sanitized failure reason, and (on success) a video player/download action.

The ticket is a short-lived HMAC/JWT-style opaque credential containing only the task ID, owner user ID, purpose (`chat-video-task-page`), issued time, and expiry. It never embeds the user's API token, channel key, or upstream video URL. The task page calls ticket-authenticated status and content endpoints. Those endpoints verify the ticket's signature, expiry, purpose, task ID, and owner before returning status or proxying bytes. They share the existing SSRF-protected video-fetch logic rather than exposing the upstream signed TOS URL.

### Observability and safety

- Record bridge mode, completion-wait result (`completed`, `timed_out`, `failed`), and task ID in admin-only log metadata. Do not record tickets, API tokens, or upstream URLs.
- Preserve existing task quota validation and settlement. A bridge request must never bypass the normal pre-consume/settlement lifecycle.
- Enforce one active bridge wait per request context and use request/context cancellation to stop only the local wait, never the underlying task.
- Apply normal per-token rate limits to the initial chat request. The task page status route gets a separate conservative ticket rate limit to prevent polling abuse.
- Maintain SQLite, MySQL, and PostgreSQL compatibility; no database schema is required unless a durable ticket revocation list is later added.

## Non-goals for the first release

- Image-to-video, first/last frame, reference video/audio, and direct uploads from chat clients.
- A general protocol that makes arbitrary asynchronous providers behave like chat.
- Automatic model discovery or automatic exposure of every Seedance task model.
- Cancellation or retry controls from the task page.

Those can be added after text-to-video bridge behavior is proven with real chat clients.

## Test strategy

1. Unit-test eligibility: disabled bridge, non-allow-listed model, unsupported model, missing task channel, and valid configured Seedance model.
2. Unit-test chat-to-task conversion: last user text accepted; empty content, image/file content, tools, and response formats rejected.
3. Controller-level tests: completed-before-deadline returns an OpenAI completion containing a signed content link; timed-out task returns a signed task-page link; failed task returns an OpenAI error; text models continue through normal relay.
4. Streaming tests: completed and timed-out paths both emit one valid SSE completion chunk and `[DONE]`.
5. Ticket tests: valid owner ticket succeeds; expired, forged, mismatched-task, and mismatched-purpose tickets are rejected; tickets cannot access another user's task.
6. Browser-level manual checks: Cherry Studio and Open WebUI receive a clickable link, and the task page transitions from progress to playable/downloadable video.

## Rollout

Ship disabled by default. An administrator first enables the bridge, sets a short wait window, and allows a single Seedance model. Verify with a non-production token and one configured task channel before exposing additional models. Keep the standard video API documented as the full-featured integration path.
