# Features

## Completed (v1)

- **Agent CA + PKI** — `rexec-agentd ca init` mints an Ed25519 CA + server cert; short-lived leaves, SHA-256 fingerprint.
- **Token-bootstrapped enrollment** — single-use join tokens; the agent signs the controller's client CSR only (trustd pattern).
- **mTLS gRPC transport** — TLS 1.3, client-cert verification; public `Enroll` + protected methods on one port.
- **Role-based authorization** — role in the client cert `O=` (`rex:reader ⊂ operator ⊂ admin`), enforced per-method (unary + stream).
- **Identity & pinning** — `rexec id` returns the stable agent id + fingerprint; pinned at enroll, re-asserted every call.
- **Streaming remote exec** — `rexec run` (operator) streams stdout/stderr live and propagates the remote exit code.
- **Destructive-op gate** — `rexec deploy` (admin) → agent `policy.yaml` (deny|allow|ask) → single-use, command-bound, 5-min live approval.
- **Cross-OS service** — `rexec-agentd service install|uninstall|start|stop|status|run` via `kardianos/service`.
- **Claude Code surface** — `/remote:enroll|id|run|deploy` slash commands + `remote-runner` fleet subagent; approval wired to `AskUserQuestion`.

## Proposed (v2)

See `docs/IMPLEMENTATION_TASKS.md` for task breakdown.

- WireGuard/SideroLink overlay for NAT traversal + WG-pubkey identity (NET-1/2).
- `--expect-fingerprint` bootstrap verification (SEC-1).
- Leaf-cert auto-rotation (SEC-2); destructive-op audit log (SEC-3).
- `--json` output mode (DX-1); multi-agent registry (DX-2); `doctor` health-check (DX-3); capability negotiation in `Info` (DX-4).
