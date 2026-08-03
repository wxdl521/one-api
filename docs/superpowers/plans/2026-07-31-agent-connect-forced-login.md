# Agent Connect Forced Login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Force a fresh The One login before any Agent Connect group/model authorization, for Hermes, MyAgents, and future pairing clients.

**Architecture:** Add one shared, request-ID-bound reauthentication endpoint that validates the short-lived Agent Connect request before delegating to existing session logout logic. The connection page calls it once before fetching options, clears its local auth state, and redirects to the normal sign-in route with the exact same-origin connection URL. Existing authenticated option/authorize APIs remain unchanged as defense in depth.

**Tech Stack:** Go 1.22, Gin, existing browser session service, React 19, TanStack Query/Router, Bun, testify, node:test.

---

### Task 1: Add a request-bound reauthentication endpoint

**Files:**
- Modify: `controller/agent_connect.go`
- Modify: `controller/agent_connect_test.go`
- Modify: `router/api-router.go`

- [ ] **Step 1: Write failing controller tests**

Add `TestAgentConnectReauthenticateRequiresLiveRequestBeforeLogout`. It creates an expired request and calls the new handler with an authenticated browser session; assert the response is unsuccessful and the refresh cookie is not cleared. Then create a live request, call the handler with the same browser credentials, and assert success, a cleared `Set-Cookie`, and a revoked current session. Do not assert token values or inspect credentials.

```go
ForceAgentConnectReauthentication(context)
response := decodeAPIResponse(t, recorder)
require.True(t, response.Success)
assert.Contains(t, recorder.Header().Get("Set-Cookie"), service.RefreshCookieName+"=")
```

- [ ] **Step 2: Verify the test fails**

Run: `go test ./controller -run Reauthenticate -count=1`

Expected: FAIL because `ForceAgentConnectReauthentication` and its route do not exist.

- [ ] **Step 3: Implement the minimal shared handler and route**

Add `ForceAgentConnectReauthentication(c *gin.Context)`. First call
`model.GetAgentConnectRequest(c.Param("request_id"))`; map errors through the
existing `writeAgentConnectError` and return without touching browser auth. For
a live request, call the existing `AuthLogout(c)` so bearer/cookie session
revocation, refresh-cookie clearing, and no-store behavior stay centralized.

Register exactly:

```go
agentConnectRoute.POST(
  "/requests/:request_id/reauthenticate",
  middleware.SessionCookieOriginGuard(),
  middleware.CriticalRateLimit(),
  controller.ForceAgentConnectReauthentication,
)
```

Keep it public because it must work before login, but require a valid opaque
request ID and the existing cookie-origin guard.

- [ ] **Step 4: Verify controller behavior**

Run: `go test ./controller -run Reauthenticate -count=1`

Expected: PASS.

### Task 2: Make every connection page log out before displaying options

**Files:**
- Modify: `web/src/features/agent-connect/api.ts`
- Modify: `web/src/features/agent-connect/index.tsx`
- Create: `web/src/features/agent-connect/forced-login.test.ts`

- [ ] **Step 1: Write failing frontend contract tests**

Create a node:test source contract that asserts the connection API calls
`POST /api/agent-connect/requests/:request_id/reauthenticate` with auth refresh
disabled. Assert the page does not enable `getAgentConnectOptions` until
reauthentication is complete, clears authenticated client state with tab sync
disabled, and navigates to `/sign-in` with the original pathname/search as
the redirect target.

```ts
assert.match(source, /forceAgentConnectReauthentication\(requestID\)/)
assert.match(source, /clearAuthenticatedClientState\(queryClient, false\)/)
assert.match(source, /to: '\/sign-in'/)
```

- [ ] **Step 2: Verify the test fails**

Run: `cd web; bun test src/features/agent-connect/forced-login.test.ts`

Expected: FAIL because no reauthentication API or page gate exists.

- [ ] **Step 3: Implement the reauthentication API and page gate**

In `api.ts`, export `forceAgentConnectReauthentication(requestID)` using the
existing Axios client with `skipAuthRefresh: true`, `skipErrorHandler: true`,
and the encoded request path. In `index.tsx`, use `useQueryClient` and a
one-shot mutation/effect for any `requestID`: call the endpoint, clear cached
and local auth state with `clearAuthenticatedClientState(queryClient, false)`,
then navigate to `/sign-in` with `redirect` equal to
`window.location.pathname + window.location.search`.

Use a `reauthenticationComplete` state only after the fresh sign-in return;
until then render the existing sign-in/loading copy and set the options query
`enabled` only when that state and `authenticated` are both true. On invalid or
expired request response, render the existing error state without clearing
browser auth.

- [ ] **Step 4: Verify frontend behavior**

Run: `cd web; bun test src/features/agent-connect/forced-login.test.ts; bun run typecheck`

Expected: PASS.

### Task 3: Validate the complete security contract

**Files:**
- Modify: files from Tasks 1-2 only

- [ ] **Step 1: Format source**

Run: `gofmt -w controller/agent_connect.go controller/agent_connect_test.go`

- [ ] **Step 2: Run focused tests**

Run: `go test ./controller ./router -run 'AgentConnect|Reauthenticate' -count=1`

Run: `cd web; bun test src/features/agent-connect/forced-login.test.ts; bun run build`

Expected: PASS.

- [ ] **Step 3: Run the regression suite**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 4: Commit, push, and deploy safely**

Stage only the Task 1-2 files and this plan with exact paths, commit with
`fix: force fresh login for Agent Connect`, and push
`codex/harden-epay-topups`. Build a new remote image while the live container
remains running, clone its Docker configuration into a prepared next container,
then stop/rename/start only after the image succeeds. Validate root, the
Hermes Skill, the MyAgents Skill, and the reauthentication route; restore the
previous container automatically if any health check fails.
