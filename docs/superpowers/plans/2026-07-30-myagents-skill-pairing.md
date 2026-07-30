# MyAgents Skill Pairing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a MyAgents agent securely configure The One from one public Skill URL, while the user only completes browser login, group/model selection, and confirmation.

**Architecture:** Extend the existing `agent_connect_requests` PKCE authorization record with a pairing mode, preserving loopback/CLI behavior. A pairing-specific create/poll exchange flow issues the existing restricted 90-day token exactly once after browser confirmation; the agent then uses MyAgents' native local configuration surface to install stable, non-default provider/MCP/Skill entries. Embed the versioned Markdown Skills into the Go binary and expose them from the gateway origin.

**Tech Stack:** Go 1.22, Gin, GORM v2 (SQLite/MySQL/PostgreSQL), React 19/TypeScript, Bun, Go `embed`, testify.

---

## File structure

- `model/agent_connect.go` — pairing mode validation, atomic pairing exchange, and shared restricted-token issuance.
- `model/agent_connect_test.go` — PKCE, state machine, expiry/cancellation/concurrency regression coverage.
- `controller/agent_connect.go` — pairing HTTP DTOs and responses; pairing confirmation no longer emits a callback.
- `controller/agent_connect_test.go` — public API contract tests for pending, authorized, and rejected polling.
- `router/api-router.go` — public pairing create/exchange routes.
- `agent-integrations/embed.go` — build-time embedding of repository-owned Markdown Skills.
- `agent-integrations/myagents/the-one-myagents-pairing/SKILL.md` — the public onboarding Skill that an agent reads first.
- `agent-integrations/myagents/the-one-gateway/SKILL.md` — the post-connection usage Skill.
- `router/skill-router.go` and `router/skill-router_test.go` — public Markdown response headers and safe content serving.
- `router/main.go` — register Skill routes before the web fallback.
- `web/src/features/agent-connect/api.ts` — discriminated authorize result for loopback versus pairing completion.
- `web/src/features/agent-connect/index.tsx` — pairing confirmation screen and the no-request starter instruction.
- `web/src/features/agent-connect/index.test.tsx` — browser confirmation behavior tests.
- `cmd/the-one-connect/main.go` and `cmd/the-one-connect/main_test.go` — retain compatibility but point any installed usage Skill at the official public Markdown URL.

### Task 1: Add a pairing mode to the existing authorization state machine

**Files:**
- Modify: `model/agent_connect.go`
- Modify: `model/agent_connect_test.go`

- [ ] **Step 1: Write failing model tests for pairing bootstrap validation**

Add table tests that call `CreateAgentConnectRequest` with `ClientKind: "myagents-skill"`, an S256 challenge, and empty redirect/state; assert success and `PairingMode == true`. Add invalid cases for a redirect URI, non-empty state, non-S256 method, and malformed challenge; assert no database row is created.

```go
requestID, request, err := CreateAgentConnectRequest(AgentConnectRequestCreate{
	ClientKind: "myagents-skill", CodeChallenge: challenge, CodeChallengeMethod: "S256",
})
require.NoError(t, err)
assert.NotEmpty(t, requestID)
assert.True(t, request.PairingMode)
assert.Empty(t, request.RedirectURI)
```

- [ ] **Step 2: Run the model tests to verify pairing is unsupported**

Run: `go test ./model -run 'Test.*AgentConnect.*Pairing' -count=1`

Expected: FAIL because pairing mode is not accepted or `PairingMode` does not exist.

- [ ] **Step 3: Implement the minimal cross-database marker and bootstrap validation**

Add `PairingMode bool` to `AgentConnectRequest` with no database default tag. Add constants `agentConnectClientMyAgentsSkill = "myagents-skill"` and validate exactly two modes:

```go
if clientKind == agentConnectClientMyAgentsSkill {
	if redirectURI != "" || state != "" {
		return errors.New("pairing requests must not include redirect URI or state")
	}
	return validateAgentConnectPKCE(codeChallenge, codeChallengeMethod)
}
```

Keep loopback validation unchanged for `myagents`; set `PairingMode` only for the Skill client. Rely on existing `AutoMigrate` behavior so SQLite, MySQL 5.7.8+, and PostgreSQL 9.6+ add the boolean-compatible column without raw dialect SQL.

- [ ] **Step 4: Run the bootstrap tests**

Run: `go test ./model -run 'Test.*AgentConnect.*Pairing' -count=1`

Expected: PASS.

- [ ] **Step 5: Write failing tests for authorization and pairing exchange**

Add deterministic tests for these observable contracts:

```go
_, err := ExchangeAgentConnectPairingRequest(requestID, verifier)
require.ErrorIs(t, err, ErrAgentConnectNotAuthorized)

_, err = ExchangeAgentConnectPairingRequest(requestID, "wrong-verifier")
require.ErrorIs(t, err, ErrAgentConnectInvalidVerifier)
```

After `AuthorizeAgentConnectRequest`, assert the right verifier creates a token with the selected group/model and `ExpiredTime` within 90 days; then assert a second call returns `ErrAgentConnectConsumed`. Add cancellation/expiry cases and a two-goroutine exchange case that observes exactly one successful token and exactly one database token row.

- [ ] **Step 6: Run the exchange tests to verify they fail**

Run: `go test ./model -run 'Test.*AgentConnect.*(PairingExchange|Concurrent)' -count=1`

Expected: FAIL because `ExchangeAgentConnectPairingRequest` does not exist.

- [ ] **Step 7: Implement atomic pairing exchange without an authorization-code return**

Extract the existing token creation and consume update into a private transaction helper taking the locked request and `now`. Implement:

```go
func ExchangeAgentConnectPairingRequest(requestID, verifier string) (*Token, error)
```

It must reject non-pairing records, validate `Status`/expiry first, verify the S256 verifier with `subtle.ConstantTimeCompare`, then call the shared issuer. Do not create or return an authorization code for pairing mode. Preserve the current loopback exchange endpoint and behavior by making it call the same shared issuer only after its code check succeeds.

- [ ] **Step 8: Run focused model tests and the package suite**

Run: `go test ./model -run 'Test.*AgentConnect' -count=1`

Run: `go test ./model -count=1`

Expected: PASS.

- [ ] **Step 9: Commit the model state-machine change**

```powershell
git add model/agent_connect.go model/agent_connect_test.go
git commit -m "feat: add PKCE Skill pairing exchange"
```

### Task 2: Expose the pairing HTTP contract and pairing completion page state

**Files:**
- Modify: `controller/agent_connect.go`
- Modify: `controller/agent_connect_test.go`
- Modify: `router/api-router.go`
- Modify: `web/src/features/agent-connect/api.ts`
- Modify: `web/src/features/agent-connect/index.tsx`
- Test: `web/src/features/agent-connect/index.test.tsx`

- [ ] **Step 1: Write failing controller tests for create, pending exchange, and completed exchange**

Add tests for:

```go
POST /api/agent-connect/pairings
{"code_challenge":"...","code_challenge_method":"S256"}
// 200: {request_id, authorization_path:"/agent-connect?request_id=...", poll_interval_seconds:2}

POST /api/agent-connect/pairings/:request_id/exchange
{"code_verifier":"..."}
// 409: {pending:true}; no key field
```

Authorize the record in the fixture, exchange it once, and assert `key`, `base_url`, selected `model`, MCP URL, and Skill metadata are present. Assert invalid verifier, expired, cancelled, and second exchange responses never include `key`.

- [ ] **Step 2: Run controller tests to verify the routes are absent**

Run: `go test ./controller -run 'Test.*AgentConnect.*Pairing' -count=1`

Expected: FAIL with route/controller handler missing.

- [ ] **Step 3: Implement DTOs and handlers without exposing an absolute untrusted URL**

Add these handler functions:

```go
func CreateAgentConnectPairing(c *gin.Context)
func ExchangeAgentConnectPairing(c *gin.Context)
```

`CreateAgentConnectPairing` must force `ClientKind: "myagents-skill"`, bind only `code_challenge` and `code_challenge_method`, and return the fixed-origin-safe relative path `"/agent-connect?request_id=" + url.QueryEscape(requestID)` plus `poll_interval_seconds: 2`. The public Skill prefixes that path with the fixed official origin; do not infer origin from host or forwarded headers. `ExchangeAgentConnectPairing` maps `ErrAgentConnectNotAuthorized` to HTTP 409 `{pending:true}`, returns a normal manifest only after successful issuance, and reuses the current error mapping with no secret-bearing error text.

Use `middleware.CriticalRateLimit()` and `anonymousRequestBodyLimit` on both public routes:

```go
agentConnectRoute.POST("/pairings", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.CreateAgentConnectPairing)
agentConnectRoute.POST("/pairings/:request_id/exchange", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.ExchangeAgentConnectPairing)
```

- [ ] **Step 4: Make browser confirmation stay on the official page for pairing mode**

After `AuthorizeAgentConnectRequest`, inspect the loaded request. For `PairingMode`, return:

```go
gin.H{"completed": true, "message": "Connection approved. Return to MyAgents."}
```

For loopback mode, preserve the existing `callback_url` response exactly. Update TypeScript to model this as `callback_url?: string; completed?: boolean`, then render a success state when `completed` is true and only call `window.location.assign` when a callback URL is supplied.

- [ ] **Step 5: Write and run frontend tests**

Mock `authorizeAgentConnectRequest` with `{ completed: true }`; assert the completion copy appears and `window.location.assign` is not called. Mock `{ callback_url: "http://127.0.0.1:1234/callback?..." }`; assert the redirect is called.

Run: `cd web; bun test src/features/agent-connect/index.test.tsx`

Expected: FAIL before the UI behavior exists, then PASS after it is implemented.

- [ ] **Step 6: Replace the no-request download prompt with the single safe instruction**

In the no-`request_id` state, show the exact text users should send to MyAgents:

```text
Read https://the-one.bolierxiang.cn/skills/myagents/SKILL.md and safely connect The One. Keep my current default provider unchanged.
```

Do not show shell, PowerShell, release binary, verifier, API key, or callback details on this page.

- [ ] **Step 7: Run backend and frontend validation**

Run: `go test ./controller ./router -run AgentConnect -count=1`

Run: `cd web; bun test src/features/agent-connect/index.test.tsx; bun run build`

Expected: PASS.

- [ ] **Step 8: Commit the HTTP and page contract**

```powershell
git add controller/agent_connect.go controller/agent_connect_test.go router/api-router.go web/src/features/agent-connect/api.ts web/src/features/agent-connect/index.tsx web/src/features/agent-connect/index.test.tsx
git commit -m "feat: add MyAgents browser pairing flow"
```

### Task 3: Publish repository-owned, embedded Skill documents

**Files:**
- Create: `agent-integrations/embed.go`
- Create: `agent-integrations/myagents/the-one-myagents-pairing/SKILL.md`
- Modify: `agent-integrations/myagents/the-one-gateway/SKILL.md`
- Create: `router/skill-router.go`
- Create: `router/skill-router_test.go`
- Modify: `router/main.go`

- [ ] **Step 1: Write failing router tests for both public documents**

Create a Gin engine, call `SetSkillRouter`, and request both URLs. Assert HTTP 200, `Content-Type` begins with `text/markdown`, `X-The-One-Skill-Version` is non-empty, and the content contains its expected frontmatter name. Assert the onboarding response does not contain `api_key`, `code_verifier`, `password`, `curl`, `.exe`, `Invoke-WebRequest`, or `the-one-connect`.

```go
assert.Contains(t, recorder.Body.String(), "name: the-one-myagents-pairing")
assert.NotContains(t, strings.ToLower(recorder.Body.String()), ".exe")
```

- [ ] **Step 2: Run the router tests to verify the documents are not public**

Run: `go test ./router -run TestSkillRouter -count=1`

Expected: FAIL because `SetSkillRouter` does not exist.

- [ ] **Step 3: Add build-time embedding and Markdown routes**

Create package `agentintegrations` in `agent-integrations/embed.go` using:

```go
//go:embed myagents/the-one-myagents-pairing/SKILL.md
var MyAgentsPairingSkill string

//go:embed myagents/the-one-gateway/SKILL.md
var MyAgentsGatewaySkill string
```

Implement `SetSkillRouter(router *gin.Engine)` with exact `GET` routes for `/skills/myagents/SKILL.md` and `/skills/myagents/the-one-gateway/SKILL.md`. Set `Content-Type: text/markdown; charset=utf-8`, `Cache-Control: public, max-age=300`, and `X-The-One-Skill-Version` to a repository constant such as `2026-07-30`. Register it in `router/main.go` before `SetWebRouter` so the SPA fallback cannot mask the document.

- [ ] **Step 4: Author the onboarding Skill with strict safety rails**

Write frontmatter:

```yaml
---
name: the-one-myagents-pairing
description: Use when a user asks MyAgents to safely connect The One from the official pairing page without changing their default provider.
---
```

Its ordered procedure must: generate a PKCE verifier/challenge in memory; POST only to `https://the-one.bolierxiang.cn/api/agent-connect/pairings`; open the returned relative authorization path under that same origin; wait/poll at the supplied interval; configure only stable origin-derived MyAgents provider/MCP/Skill entries using the returned secret configuration channel; verify model/MCP; retry idempotently. It must explicitly prohibit all executable downloads, source clones, shell bootstrap commands, external installers, secret/browser credential inspection or disclosure, automatic login/2FA entry, setting a default provider, and changes to unrelated MyAgents configuration. It must abort if native configuration is unavailable.

- [ ] **Step 5: Update the usage Skill and source references**

Update `the-one-gateway` to tell agents to use the configured provider and read-only MCP tools, and to rerun the public pairing Skill when reconnecting. Remove its instruction to run `the-one-connect`. In `controller/agent_connect.go` and `cmd/the-one-connect/main.go`, change the usage Skill source from `QuantumNous/the-one` to `https://the-one.bolierxiang.cn/skills/myagents/the-one-gateway/SKILL.md`; update `cmd/the-one-connect/main_test.go` accordingly.

- [ ] **Step 6: Run document, router, and static checks**

Run: `go test ./router ./cmd/the-one-connect -run 'Test(SkillRouter|.*Skill)' -count=1`

Run: `go test ./agent-integrations/... -count=1`

Run: `rg -n "(Invoke-WebRequest|curl|\.exe|the-one-connect)" agent-integrations/myagents/the-one-myagents-pairing/SKILL.md`

Expected: test PASS; the final `rg` returns no matches.

- [ ] **Step 7: Commit the embedded Skills**

```powershell
git add agent-integrations controller/agent_connect.go cmd/the-one-connect/main.go cmd/the-one-connect/main_test.go router/skill-router.go router/skill-router_test.go router/main.go
git commit -m "feat: publish safe MyAgents pairing Skill"
```

### Task 4: Validate the complete security and compatibility contract

**Files:**
- Modify: `model/agent_connect_test.go`
- Modify: `controller/agent_connect_test.go`
- Modify: `router/skill-router_test.go`
- Modify: `docs/superpowers/specs/2026-07-30-myagents-skill-pairing-design.md`

- [ ] **Step 1: Add regression tests for no token leakage**

For every rejected exchange fixture (pending, invalid verifier, cancelled, expired, consumed), serialize the controller response and assert it does not contain the actual issued fixture key nor a `"key"` JSON property. For a pairing authorization response assert it does not contain `callback_url` or `authorization_code`; for loopback assert callback behavior still contains no key.

- [ ] **Step 2: Run security regression tests**

Run: `go test ./model ./controller ./router -run '(AgentConnect|SkillRouter)' -count=1`

Expected: PASS.

- [ ] **Step 3: Verify all supported database paths and preserved loopback behavior**

Run the existing migration-backed model tests using the project’s normal SQLite test setup, then run the full related packages:

```powershell
go test ./model ./service ./controller ./router ./middleware ./cmd/the-one-connect -count=1
```

Expected: PASS. The test suite must exercise `AutoMigrate` with the new `PairingMode` field; no raw SQL or dialect branch is introduced.

- [ ] **Step 4: Mark the design acceptance details as implemented**

Append an `## Implementation verification` section to the approved design with the exact local commands run, the public document URLs, and the statement that legacy loopback requests remain supported. Do not claim browser/manual acceptance until it has actually been performed after deployment.

- [ ] **Step 5: Commit verification coverage and documentation**

```powershell
git add model/agent_connect_test.go controller/agent_connect_test.go router/skill-router_test.go docs/superpowers/specs/2026-07-30-myagents-skill-pairing-design.md
git commit -m "test: cover MyAgents pairing security boundaries"
```

### Task 5: Ship without changing GitHub main and perform a manual pairing acceptance pass

**Files:**
- Modify: `docs/superpowers/specs/2026-07-30-myagents-skill-pairing-design.md`

- [ ] **Step 1: Inspect the scoped diff and run final build checks**

Run:

```powershell
git status --short
git diff origin/main...HEAD -- model controller router agent-integrations cmd/the-one-connect web/src/features/agent-connect docs/superpowers
go test ./model ./service ./controller ./router ./middleware ./cmd/the-one-connect -count=1
cd web; bun run build
```

Expected: only intentional paths are staged/committed; unrelated user worktree changes remain untouched; all checks pass.

- [ ] **Step 2: Push only the feature branch**

Run:

```powershell
git push origin codex/harden-epay-topups
git log --oneline origin/main..HEAD
```

Expected: the feature commits are visible on the branch and `origin/main` is unchanged.

- [ ] **Step 3: Build a new immutable server image while the current container remains live**

On the already-open server terminal, fast-forward only `/opt/the-one` on `codex/harden-epay-topups`, record the current image/container as a rollback target, and build `the-one:next-<short-sha>`. Do not edit, reset, merge, or force-push `main`; do not print environment secrets.

- [ ] **Step 4: Start the replacement container with the existing ports, mounts, command, and environment**

Use the Docker API/container inspection to clone the existing runtime configuration. Start the new container under a temporary name, wait for `https://the-one.bolierxiang.cn/` and both public Skill URLs to return 200, then swap names only after successful health checks. Keep the previous container stopped under a dated rollback name. If any check fails, remove only the new temporary container and start the recorded rollback container.

- [ ] **Step 5: Manually verify the user-visible flow**

Open `https://the-one.bolierxiang.cn/skills/myagents/SKILL.md` and verify Markdown plus the version header. In MyAgents, send the exact one-line instruction from Task 2. Confirm the agent opens the official authorization page, the signed-in user can select a group/model and approve, and the agent reports configured provider/MCP/usage Skill without exposing a key or changing the existing default provider. Also cancel one pairing session and verify the agent reports failure without retrying a secret-bearing action.

- [ ] **Step 6: Record deployment verification and commit only the documentation update**

Add the deployed image tag, UTC verification time, tested URLs, and acceptance result to `## Implementation verification` in the design document.

```powershell
git add docs/superpowers/specs/2026-07-30-myagents-skill-pairing-design.md
git commit -m "docs: record MyAgents pairing deployment verification"
git push origin codex/harden-epay-topups
```

## Self-review

- Spec coverage: Tasks 1-2 implement PKCE pairing creation, browser confirmation, pending polling, exactly-once 90-day restricted token issuance, and legacy loopback compatibility. Task 3 implements the versioned public Skill and gateway serving. Task 4 verifies secrecy, state transitions, and database migration behavior. Task 5 preserves GitHub `main`, deploys safely with a rollback container, and performs user-visible acceptance.
- Placeholder scan: this plan has no deferred implementation placeholders; every task names files, tests, commands, and required API behavior.
- Type consistency: the endpoint and model signatures used throughout are `CreateAgentConnectPairing`, `ExchangeAgentConnectPairing`, `ExchangeAgentConnectPairingRequest`, `PairingMode`, and `the-one-myagents-pairing`; later tasks use the same names.
