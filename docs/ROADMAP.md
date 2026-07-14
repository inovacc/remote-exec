# Roadmap

## Current Status
**Overall Progress:** ~5% — Scaffolded (monorepo: `rexec-agentd` daemon + `rexec` CLI, builds clean). Design + Talos security research committed.

See `docs/DESIGN.md` for the full architecture and `docs/research/TALOS-SECURE-COMMS.md` for the security derivation.

## Phases

### P0 — Scaffold [DONE]
- [x] substrate monorepo: `cmd/rexec-agentd` (daemon) + `cmd/rexec` (controller CLI)
- [x] Taskfile, golangci v2, goreleaser, GitHub CI, BSD-3, sqlite platform aggregator
- [x] `go build ./...` + `go vet ./...` clean

### P1 — PKI + enrollment [NOT STARTED]
- [ ] `internal/pki`: agent CA mint (Ed25519), CSR sign, leaf rotation, `sha256(cert.Raw)` fingerprint
- [ ] Join-token issuance (`rexec-agentd token new`) — short-lived, single-use
- [ ] `Enroll` RPC — agent signs the client CSR only, returns `{cert, caPEM, agentID, fingerprint}`
- [ ] `talosconfig`-style controller credential `~/.rexec/config.yaml`

### P2 — mTLS transport + interceptors [NOT STARTED]
- [ ] `internal/transport`: gRPC over TLS 1.3, `RequireAndVerifyClientCert`
- [ ] `authenticate` (peer cert → identity + role from `O=`) and `authorize` (per-method role table) interceptors

### P3 — Exec + streaming [NOT STARTED]
- [ ] `internal/proto` (`rexec.v1`): `Identity`, `Info`, `Exec`, `Deploy`
- [ ] Server-streaming `ExecChunk{stdout,stderr,exit_code,needs_approval}` — live build/deploy logs

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
