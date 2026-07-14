# Backlog

Deferred / future work, distilled from `docs/DESIGN.md` §7 and the Talos research.

## Deferred to v2
- **WireGuard overlay (SideroLink pattern).** `wireguard-go` userspace mesh so agents behind
  NAT are reachable out-of-band; WG public key becomes a second cryptographic identity.
- **Leaf-cert auto-rotation daemon.** Background rotation of short-lived leaves before expiry
  (Talos rotates ~1y leaves against a ~10y CA).
- **Multi-agent fleet registry.** Controller-side discovery/listing across many enrolled agents
  (reuse the `instances-manager` / corral provider+registry pattern).

## Tech debt / hardening
- Reuse **weaver** `internal/identity` + `tlsutil` for cert→agentID rather than re-deriving.
- Reuse **agentbox** allow-list + read-only credential seed for the exec sandbox.
- Payload signing on RPC requests (weaver `lib/signature`) for non-repudiation of destructive ops.
- Audit log of every destructive op (who/role/fingerprint/op/approval-token) — append-only.

## Ideas / stretch
- `rexec doctor` health-check command (corral `Doctor` pattern) — cert validity, clock skew, reachability.
- Capability negotiation in `Info` (toolchains present: go, xcodebuild, msbuild…).
- Session recording / replay of remote exec streams.
- OTel spans across the controller→agent boundary (substrate `--otel` already wired).
