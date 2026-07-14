# Roadmap

## Current Status
**Overall Progress:** ~45% — P0–P3 done. Secure channel + live streaming remote execution work: an operator streamed `go version` off a remote agent while a reader was refused `Exec` at the transport. Next: P4 destructive-op gate (agent policy + `NeedsApproval`).

See `docs/DESIGN.md` for the full architecture and `docs/research/TALOS-SECURE-COMMS.md` for the security derivation.

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

### P4 — Destructive-op gate [NOT STARTED]
- [ ] Role model `rex:reader ⊂ rex:operator ⊂ rex:admin`
- [ ] Agent `policy.yaml` (`destructive: deny|allow|ask`, allow-list)
- [ ] `NeedsApproval` live-approval flow (one-time token)

### P5 — Cross-OS service [NOT STARTED]
- [ ] `kardianos/service` install/uninstall/run on macOS, Linux, Windows

### P6 — Claude Code surface [NOT STARTED]
- [ ] `rexec` skills/commands (`/remote:enroll`, `/remote:id`, `/remote:run`) + fleet subagent
- [ ] Human-in-the-loop approval wired to `AskUserQuestion`

### P7 — v2 WireGuard overlay [DEFERRED — see BACKLOG]
- [ ] SideroLink-style `wireguard-go` mesh + WG-pubkey second identity
