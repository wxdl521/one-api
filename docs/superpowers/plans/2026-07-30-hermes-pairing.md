# Hermes Skill Pairing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish a safe, one-instruction Hermes connection Skill that preserves the existing default model.

**Architecture:** Generalize the existing MyAgents Skill pairing client kind and manifest source while reusing its PKCE/token state machine. Embed and serve Hermes onboarding and usage Markdown documents; their instructions use only Hermes-native configuration and a named, non-default provider.

**Tech Stack:** Go 1.22, Gin, GORM v2, Go embed, testify.

---

### Task 1: Add Hermes pairing coverage and client validation

**Files:**
- Modify: `model/agent_connect_test.go`
- Modify: `model/agent_connect.go`

- [ ] Write a failing model test that creates a `hermes-skill` PKCE pairing and asserts `PairingMode` is true; verify an unknown skill client fails.
- [ ] Run `go test ./model -run Hermes -count=1` and observe the expected failure.
- [ ] Permit only `myagents-skill` and `hermes-skill` in the existing pairing branch, retaining strict no-redirect/no-state/S256 validation.
- [ ] Run `go test ./model -run 'AgentConnect.*(Pairing|Hermes)' -count=1`.

### Task 2: Make pairing manifest source client-aware

**Files:**
- Modify: `controller/agent_connect.go`
- Modify: `controller/agent_connect_test.go`

- [ ] Write a failing controller test that creates a Hermes pairing request, authorizes it, exchanges it, and expects the direct Hermes usage Skill URL.
- [ ] Run `go test ./controller -run Hermes -count=1` and observe the expected failure.
- [ ] Accept `client_kind` only from `{myagents-skill, hermes-skill}` in pairing creation; persist it and select the manifest Skill source from the consumed token's request client kind without changing the MyAgents result.
- [ ] Run `go test ./controller -run 'AgentConnect.*(Pairing|Hermes)' -count=1`.

### Task 3: Embed and expose Hermes Skills

**Files:**
- Modify: `agent-integrations/embed.go`
- Create: `agent-integrations/hermes/the-one-hermes-pairing/SKILL.md`
- Create: `agent-integrations/hermes/the-one-gateway/SKILL.md`
- Modify: `router/skill-router.go`
- Modify: `router/skill-router_test.go`

- [ ] Write failing router tests for both Hermes public URLs and the onboarding safety contract.
- [ ] Run `go test ./router -run Hermes -count=1` and observe the expected failure.
- [ ] Embed the two Markdown documents and add exact GET routes; preserve all MyAgents routes.
- [ ] Write the onboarding document to create only the stable named provider/MCP/Skill, use `THE_ONE_API_KEY` secret storage, compare the default model before/after, and prohibit executable/credential actions.
- [ ] Write the usage document for models, connection status, usage, and reconnect.
- [ ] Run `go test ./router -run '(SkillRouter|Hermes)' -count=1`.

### Task 4: Validate, publish, and deploy

**Files:**
- Modify: files from Tasks 1-3 only

- [ ] Run `gofmt -w` on modified Go files.
- [ ] Run `go test ./model ./controller ./router -count=1` then `go test ./... -count=1`.
- [ ] Commit only Hermes files and force-added design documents; push `codex/harden-epay-topups`.
- [ ] Build a new remote Docker image while the current container remains running; retain the previous container as a stopped rollback target; swap only after public Skill and root health checks pass.
