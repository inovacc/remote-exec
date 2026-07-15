# remote-exec
<!-- rev:002 -->

> Secure, cross-OS remote execution for Claude Code. A controller — driven by a Claude Code
> subagent fleet, skills, and commands — discovers a remote machine by id, opens a
> mutually-authenticated channel, and runs analysis / build / deploy actions on it, with a
> cryptographically-enforced gate on destructive operations. Security model derived from
> [Talos Linux](https://github.com/siderolabs/talos).

## Why

Claude Code runs on one machine but often needs to act on another — build on Linux, notarize on
macOS, test on Windows. `remote-exec` replaces ad-hoc SSH with a principled channel: mutual TLS,
per-instance identity, short-lived certs, role-based authorization, and human-in-the-loop
approval for anything destructive.

## Components

| Binary | Role |
|--------|------|
| `rexec-agentd` | The agent daemon. Runs as an OS service on macOS/Linux/Windows, terminates mTLS, serves the gRPC API, enforces policy. |
| `rexec` | The controller CLI Claude Code drives: `agent enroll`, `agent identity`, `agent info`, `exec run`, `exec deploy`. |

## Quick start

On the **agent** host:
```bash
rexec-agentd ca init                     # mint the agent CA + server cert (prints agent id + fingerprint)
# optional destructive-op policy:
cp docs/policy.example.yaml <data-dir>/policy.yaml
rexec-agentd service install             # install as an OS service (launchd/systemd/Windows)
rexec-agentd service start               # or run directly: rexec-agentd serve --listen 127.0.0.1:50000
rexec-agentd token new --role rex:operator   # issue a single-use join token
```

On the **controller** (where Claude Code runs):
```bash
rexec agent enroll --endpoint <host:port> --token <token>  # pins id+fingerprint, writes ~/.rexec/config.yaml
rexec agent identity                     # ask the agent for its id, re-assert the pin
rexec agent info                         # host os/arch/version
rexec exec run go build ./...            # non-destructive: streams output live
rexec exec deploy ./release.sh           # destructive: goes through the approval gate
```

## Security model (Talos-derived)

- **PKI + enrollment (trustd pattern):** one Ed25519 CA per agent; a single-use join token lets
  the agent sign the controller's **client** cert only. The controller credential mirrors
  `talosconfig` (CA + client cert/key + endpoints + pinned id/fingerprint).
- **mTLS + RBAC gate:** gRPC over TLS 1.3 with client-cert verification. The caller's role lives
  in the cert Subject `O=` (`rex:reader ⊂ operator ⊂ admin`); a per-method interceptor enforces
  it. A reader cert physically cannot invoke `exec run`; only an admin cert reaches `exec deploy`.
- **Identity + pinning:** stable agent id (host UUID) pinned to `sha256(server-cert)`; every call
  re-asserts it and errors on mismatch.
- **Destructive-op gate:** admin role → agent `policy.yaml` (`deny|allow|ask`) → single-use,
  command-bound, 5-minute live approval surfaced to the human via `AskUserQuestion`.

See [docs/DESIGN.md](docs/DESIGN.md), [docs/research/TALOS-SECURE-COMMS.md](docs/research/TALOS-SECURE-COMMS.md),
and [docs/CLAUDE-INTEGRATION.md](docs/CLAUDE-INTEGRATION.md).

## Build / test

```bash
task build       # go build ./...
task test        # fast tests
task test:full   # race + coverage
task proto       # regenerate gRPC from proto/ (needs protoc + protoc-gen-go[-grpc])
```

## Status

v1 complete (secure channel, streaming exec, destructive-op gate, cross-OS service, Claude Code
surface). See [docs/ROADMAP.md](docs/ROADMAP.md). WireGuard/SideroLink overlay is the v2 stretch
([docs/BACKLOG.md](docs/BACKLOG.md)).

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright (c) 2026 dyammarcano.
