# Roadmap

## Current Status
**Overall Progress:** v1 complete (P0–P6). Everything the original brief named is working and verified cross-process: cross-OS secure agent, Talos-style mTLS + PKI enrollment, ask-the-instance-for-its-id (fingerprint-pinned), streaming remote build/analyze/deploy, and the destructive-op approval gate — driven from Claude Code via slash commands + a fleet subagent. Only P7 (WireGuard overlay) remains, a documented **v2** stretch.

See `docs/DESIGN.md` for the full architecture and `docs/research/TALOS-SECURE-COMMS.md` for the security derivation.

## Test Coverage (`go test -cover`)

| Package | Coverage | | Package | Coverage |
|---------|----------|-|---------|----------|
| internal/execute | 96.8% | | internal/pki | 76.5% |
| internal/policy | 89.8% | | internal/clientconfig | 70.8% |
| internal/enroll | 81.8% | | internal/transport | 57.1% |
| internal/token | 79.3% | | internal/authz | 20.5%¹ |
| internal/identity | 78.6% | | internal/agentserver | 0.0%¹ |

¹ `authz` interceptors and the whole `agentserver` are exercised through `internal/transport`'s
bufconn round-trip tests; Go attributes that coverage to `transport`, not to the package under
test. `cmd/*` CLIs and generated `internal/proto/*` have no in-package unit tests (verified via
cross-process smoke). Raising direct coverage on `authz`/`agentserver` is tracked as TST-1 in
`docs/IMPLEMENTATION_TASKS.md`. **Total: 23.9%.**

## Phases

### P0 — Scaffold [DONE]
- [x] substrate monorepo: `cmd/rexec-agentd` (daemon) + `cmd/rexec` (controller CLI)
- [x] Taskfile, golangci v2, goreleaser, GitHub CI, BSD-3, sqlite platform aggregator
- [x] `go build ./...` + `go vet ./...` clean

### P1 — PKI + enrollment [DONE — transport-agnostic libraries]
- [x] `internal/pki`: Ed25519 CA mint, CSR sign (client/server, roles→`O=`), `sha256(cert.Raw)` fingerprint
- [x] `internal/identity`: stable persisted agent UUID
- [x] `internal/token`: short-lived, single-use join tokens (file-backed, flock, clock-injectable)
- [x] `internal/enroll`: enrollment service — signs the client CSR **only**, returns `{cert, caPEM, agentID, fingerprint}`
- [x] `internal/clientconfig`: `talosconfig`-style credential + mTLS `ClientTLS()`
- [x] `rexec-agentd ca init` + `rexec-agentd token new` (runnable; verified end-to-end)
- [ ] The **`Enroll` RPC** itself is P2 (needs the gRPC transport); the service logic above is done and unit-tested. Leaf auto-rotation → BACKLOG.

### P2 — mTLS transport + interceptors [DONE]
- [x] `internal/proto/rexec/v1`: `Agent` gRPC service (`Enroll`, `Identity`, `Info`) generated via `task proto`
- [x] `internal/transport`: gRPC over TLS 1.3, `VerifyClientCertIfGiven` (public `Enroll` + protected mTLS on one port); `ServerCreds`/`ClientCreds`/`Dial`/`Enroll`
- [x] `internal/authz`: role from cert `O=`, per-method `Table`, `UnaryInterceptor` — the destructive-op gate's enforcement point
- [x] `internal/agentserver`: `Agent` service impl over the enroll service + identity
- [x] `rexec enroll` / `rexec id` (controller) and `rexec-agentd` serve path
- [x] bufconn round-trip tests: enroll→Identity over mTLS; **reader cert refused an admin method**; no-cert refused a protected method
- [x] cross-process smoke verified (see Current Status)

### P3 — Exec + streaming [DONE]
- [x] proto: `Exec`/`Deploy` (server-streaming) + `ExecChunk{stdout,stderr,exit_code,needs_approval}` + `ApprovalRequest`
- [x] `internal/execute`: streaming command runner (serialized emit, exit-code propagation)
- [x] `agentserver`: `Exec` (operator) / `Deploy` (admin) handlers, shared `runStream`
- [x] `authz.StreamInterceptor` + table entries (`Exec`→operator, `Deploy`→admin); registered via `ChainStreamInterceptor`
- [x] `rexec run <cmd>` — streams stdout/stderr live, propagates remote exit code
- [x] tests: execute unit tests (subprocess helper); bufconn Exec streaming for operator; **reader refused Exec**
- [x] cross-process smoke: operator streamed `go version` off the agent; reader denied
- [ ] `needs_approval` is defined in the proto but wired in P4 (Deploy currently runs as an admin-gated Exec)

### P4 — Destructive-op gate [DONE]
- [x] `internal/policy`: `Policy` (`destructive: deny|allow|ask`, `allow`/`deny` lists), `Evaluate`, safe-default `Load`; `Grants` (in-memory, single-use, TTL, command-bound approvals)
- [x] `Deploy` three-stage gate: admin role (authz) → policy Evaluate → run / deny / `NeedsApproval`
- [x] Approval round-trip via `ExecRequest.approval_id`; single-use, 5-min TTL, command-matched
- [x] `rexec deploy <cmd>`: prints machine-parseable `APPROVAL_REQUIRED approval_id=…` (for a fleet to surface via `AskUserQuestion`), plus `--approval <id>` and `--yes`
- [x] `docs/policy.example.yaml`; `rexec-agentd` logs `destructive_policy` on start
- [x] tests: policy Evaluate matrix + Grants; bufconn Deploy allow/deny/ask + approval round-trip (single-use)
- [x] cross-process smoke: ask→approve→run, reuse rejected, `--yes` one-shot, deny blocked

### P5 — Cross-OS service [DONE]
- [x] `kardianos/service` lifecycle adapter (`program` Start/Stop → `serveAgent` with a cancelable context)
- [x] `rexec-agentd service install|uninstall|start|stop|status|run` (launchd/systemd/Windows-service)
- [x] `service run` is the manager entrypoint; install records `service run --data-dir --listen` args
- [x] build/vet clean; command wiring verified (actual install needs admin + mutates the OS, so not run in CI)

### P6 — Claude Code surface [DONE]
- [x] slash commands: `/remote:enroll`, `/remote:id`, `/remote:run`, `/remote:deploy` (in `.claude/commands/remote/`)
- [x] `remote-runner` fleet subagent (`.claude/agents/`) — drives one agent end-to-end, one per target host
- [x] destructive approval wired to `AskUserQuestion` (the `deploy` command/agent consume `APPROVAL_REQUIRED`)
- [x] `docs/CLAUDE-INTEGRATION.md`: handshake, gate, and cross-OS fleet pattern

### P7 — v2 WireGuard overlay [DEFERRED — see BACKLOG]
- [ ] SideroLink-style `wireguard-go` mesh + WG-pubkey second identity
