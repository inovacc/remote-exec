# Milestones
<!-- rev:002 -->

## v1.0 — Secure cross-OS remote execution [COMPLETE]

Everything the original brief named, delivered and verified cross-process. Maps to phases
P0–P6 in `docs/ROADMAP.md`.

Goals:
- [x] Cross-OS agent daemon (`rexec-agentd`) installable as an OS service (mac/linux/windows)
- [x] Talos-derived secure channel: Ed25519 CA, token-bootstrapped enrollment, mTLS (TLS 1.3)
- [x] Per-instance identity + fingerprint pinning (`rexec agent identity`)
- [x] Role-based authorization from the client cert (`rex:reader ⊂ operator ⊂ admin`)
- [x] Streaming remote execution (`rexec exec run`) — build/test/analyze
- [x] Destructive-op gate (`rexec exec deploy`): policy + single-use live approval
- [x] Claude Code surface: `/remote:*` commands + `remote-runner` fleet subagent

Test coverage (logic packages): execute 96.8%, policy 89.8%, enroll 81.8%, token 79.3%,
identity 78.6%, pki 76.5%, clientconfig 70.8%, transport 57.1%. (`cmd/*` CLIs and generated
proto are exercised via cross-process smoke tests, not in-package unit coverage.)

No git tags yet — tag `v1.0.0` when releasing.

## v2.0 — Overlay + hardening [PLANNED]

Goals (from `docs/BACKLOG.md`):
- [ ] WireGuard / SideroLink-style overlay (`wireguard-go`) for NAT traversal + WG-pubkey identity (P7)
- [ ] Optional `--expect-fingerprint` on `rexec agent enroll` (close the bootstrap MITM window)
- [ ] Leaf-cert auto-rotation daemon
- [ ] Multi-agent fleet registry (list/discover enrolled agents)
- [ ] Append-only audit log of destructive operations
- [ ] `--json` output mode for `token new` / `enroll` / `id` (clean machine parsing)

Coverage target for v2: ≥80% on all `internal/*` logic packages.
