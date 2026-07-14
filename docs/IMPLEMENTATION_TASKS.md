# Implementation Tasks

Granular tasks for the remaining (v2 / hardening) work. v1 (P0–P6) is delivered — see
`docs/ROADMAP.md`. Effort: S (<½ day), M (~1–2 days), L (multi-day).

## Domain: Security hardening

| ID | What | Files | Deps | Effort |
|----|------|-------|------|--------|
| SEC-1 | `--expect-fingerprint` on `rexec enroll`; verify agent server cert during bootstrap instead of `InsecureSkipVerify` | `internal/transport/transport.go`, `cmd/rexec/controllercmds.go` | — | M |
| SEC-2 | Leaf-cert auto-rotation: background re-issue of client/server leaves before expiry | `internal/pki`, new `internal/rotate` | — | M |
| SEC-3 | Append-only audit log of destructive ops (who/role/fingerprint/op/approval) | `internal/agentserver`, new `internal/audit` | — | M |
| SEC-4 | Sign RPC requests for non-repudiation of destructive ops (weaver `lib/signature` pattern) | `internal/transport`, `proto/` | SEC-3 | L |

## Domain: Overlay network (v2)

| ID | What | Files | Deps | Effort |
|----|------|-------|------|--------|
| NET-1 | `wireguard-go` userspace overlay (SideroLink pattern) for agents behind NAT | new `internal/overlay` | — | L |
| NET-2 | WG public key as a second cryptographic identity; surface in `Info`/`Identity` | `internal/identity`, `proto/`, `internal/agentserver` | NET-1 | M |

## Domain: Fleet / DX

| ID | What | Files | Deps | Effort |
|----|------|-------|------|--------|
| DX-1 | `--json` output for `token new` / `enroll` / `id` (clean parsing for a fleet) | `cmd/rexec-agentd/agentcmds.go`, `cmd/rexec/controllercmds.go` | — | S |
| DX-2 | Multi-agent registry: persist + list enrolled agents controller-side (sqlite platform) | `cmd/rexec/…`, `internal/clientconfig` | — | M |
| DX-3 | `rexec-agentd doctor` health-check (cert validity, clock skew, reachability) | `cmd/rexec-agentd` | — | S |
| DX-4 | Capability negotiation in `Info` (toolchains present: go, xcodebuild, msbuild…) | `internal/agentserver`, `proto/` | — | S |

## Domain: Test coverage

| ID | What | Files | Deps | Effort |
|----|------|-------|------|--------|
| TST-1 | Direct unit tests for `internal/authz` interceptors (raise from 20.5% own-package) | `internal/authz/authz_test.go` | — | S |
| TST-2 | Round-trip test for `pki.LoadCA` error paths + `clientconfig.ClientTLS` edge cases | respective `_test.go` | — | S |
| TST-3 | CLI-level smoke tests scripted under `test/` (enroll→run→deploy) as Go integration tests | new `test/` | — | M |
