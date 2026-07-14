# remote-exec — Design

> Secure, cross-OS remote execution for Claude Code. A controller (driven by a Claude
> Code subagent fleet / skills / commands) discovers a remote instance by ID, opens a
> mutually-authenticated channel, and runs analysis / build / deploy actions — with a
> cryptographically-enforced gate on destructive operations.

Date: 2026-07-14 · Status: Draft · Author: dyammarcano

## 1. Problem & goals

Claude Code runs on one machine but often needs to act on another — build on Linux,
notarize on macOS, test on Windows. Today that means ad-hoc SSH, brittle scripts, and
no principled control over *what* the remote is allowed to do. We want:

- **G1 — Cross-OS agent.** One agent binary runs as a service on macOS, Linux, Windows.
- **G2 — Secure by construction.** No plaintext channel, no shared password. Mutual TLS,
  per-instance identity, short-lived leaf certs — the Talos security model, minus the
  Kubernetes/etcd/COSI weight.
- **G3 — Identity & discovery.** A controller can "ask for the ID of the other instance"
  and pin it: stable `agentID` (host UUID) + cert fingerprint.
- **G4 — Destructive-op gate.** The channel itself decides whether a caller may run
  destructive actions — not a flag the caller sets. Enforced from the client cert's role.
- **G5 — Claude Code native.** The controller is a CLI + a set of skills/commands/subagents
  a fleet can invoke; long-running output streams back live.

Non-goals (v1): multi-node scheduling, a full mesh control-plane, replacing SSH globally,
GUI. WireGuard overlay is designed-for but deferred to v2.

## 2. Architecture

```
Claude Code (subagent fleet, skills, commands)
        │  invokes
        ▼
  rexec  (controller CLI)  ──mTLS gRPC──▶  rexec-agentd  (daemon on remote host)
   • enroll   • id/whoami                    • Exec (streamed stdout/stderr/exit)
   • run/build/deploy/analyze                • Info / Identity
   • ls (registry of known agents)           • policy engine (allow/deny + destructive)
        │                                     • kardianos/service lifecycle
        ▼
  local registry  (agentID → {fingerprint, endpoint, role, wg-pubkey})
```

Two binaries, one Go module (**monorepo layout**, module `github.com/inovacc/remote-exec`):

| Path | Role |
|------|------|
| `cmd/rexec-agentd` | The daemon. Runs as an OS service; terminates mTLS; serves the gRPC API; enforces policy. |
| `cmd/rexec` | The controller CLI Claude Code drives. Enrolls, resolves an agent by ID, runs actions. |
| `internal/proto` | gRPC service + message definitions (`rexec.v1`). |
| `internal/pki` | CA mint, CSR sign, cert rotation, fingerprinting. |
| `internal/transport` | gRPC server/client wiring, TLS config, interceptors. |
| `internal/policy` | RBAC role model + per-agent allow/deny + destructive gate. |
| `internal/registry` | Controller-side map of enrolled agents (SQLite, weaver-style). |
| `internal/identity` | agentID (host UUID) + fingerprint derivation. |
| `internal/platform` | (substrate aggregator) service install, config, otel. |

## 3. Security model (Talos-derived)

Full derivation: `docs/research/TALOS-SECURE-COMMS.md`.

### 3.1 PKI
- One **agent CA** per host (Ed25519), minted offline at `rexec-agentd init`. Long-lived (~10y).
- **Server leaf** (agent's own TLS identity) and **client leaves** (controller creds) chain to it.
  Leaves short-lived (~24h–1y) and auto-rotated; CA never leaves the host in private form.
- The controller credential mirrors Talos's `talosconfig`: `{ caPEM, clientCert, clientKey,
  endpoints }` — a single portable file (`~/.rexec/config.yaml`).

### 3.2 Enrollment (trustd pattern)
No pre-shared certs. To enroll:
1. Operator runs `rexec-agentd token new` → short-lived, single-use **join token**.
2. Controller `rexec enroll <endpoint> --token <t>`: generates its keypair + CSR, calls
   `Enroll(csr, token)` over a bootstrap TLS channel.
3. Agent validates the token, signs **only the client CSR** with the requested (or default)
   role, returns `{signedCert, caPEM, agentID, fingerprint}`. Controller pins it in the registry.

The agent signs client certs; the controller signs nothing. A token grants at most the role
the operator scoped it to.

### 3.3 Authentication & the destructive-op gate
Transport: **gRPC over TLS 1.3, `RequireAndVerifyClientCert`.** Two chained interceptors:

1. `authenticate` — pull the verified client cert via `peer.FromContext`; extract identity
   (CN) and **role from the cert Subject `O=`** field.
2. `authorize` — look up `requiredRole[fullMethod]` in a static table; reject if the cert's
   role is insufficient.

Roles: `rex:reader` (Info/Identity/read-only analyze) ⊂ `rex:operator` (build/test/non-destructive)
⊂ `rex:admin` (deploy, delete, arbitrary destructive). **A reader-cert controller physically
cannot invoke a destructive RPC** — authz is on the cryptographic identity, not a request flag.

Defense in depth, three independent checks a destructive op must pass:
1. **Role** — cert `O=` ≥ `rex:admin`.
2. **Agent policy** — the op matches the agent's local allow-list (`policy.yaml`: `destructive: deny|allow|ask`, `allow: [...]`).
3. **Live approval** (when `ask`) — agent returns `NeedsApproval`; controller must re-call with
   a one-time approval token. This is the "ask if it can perform destructive operations" flow —
   the *agent* owns the answer, and Claude Code surfaces it to the human.

### 3.4 API surface — typed RPCs, not a generic shell
Discrete methods so authorization is meaningful per operation:

```proto
service Agent {
  rpc Identity(IdentityReq) returns (IdentityResp);          // reader — "who are you / your ID"
  rpc Info(InfoReq)         returns (InfoResp);              // reader — os, arch, capabilities
  rpc Enroll(EnrollReq)     returns (EnrollResp);            // bootstrap channel only
  rpc Exec(ExecReq)         returns (stream ExecChunk);      // operator/admin per policy
  rpc Deploy(DeployReq)     returns (stream ExecChunk);      // admin + destructive gate
}
message ExecChunk { oneof m { bytes stdout = 1; bytes stderr = 2; int32 exit_code = 3;
                              ApprovalRequest needs_approval = 4; } }
```
Server-streaming `ExecChunk` streams build/deploy logs live back to the fleet.

### 3.5 Identity & discovery
`agentID` = host machine UUID (stable across restarts). Fingerprint = `sha256(cert.Raw)`.
`rexec id <endpoint>` returns both; the controller pins `agentID → fingerprint` at enroll and
re-asserts on every reconnect (TOFU + pinning). Optional WireGuard public key as a second
identity for the v2 overlay.

## 4. Cross-OS

- **Service lifecycle** via `kardianos/service` (reused from the `daemon` project) — installs
  `rexec-agentd` as a launchd/systemd/Windows-service.
- **Transport is pure-Go** (`crypto/tls`, `crypto/x509`, `google.golang.org/grpc`) — no CGO,
  builds for all three OSes from one toolchain.
- **v2 overlay:** `wireguard-go` userspace (SideroLink-style) for agents behind NAT — a second
  cryptographic identity and out-of-band reachability. Designed-for; not built in v1.

## 5. Claude Code integration

- `rexec` CLI is the primitive. On top: skills/commands (`/remote:run`, `/remote:enroll`,
  `/remote:id`) and a subagent that a fleet dispatches to drive a remote build/deploy.
- The destructive gate maps to a human-in-the-loop: agent says `NeedsApproval` → the fleet
  surfaces it via `AskUserQuestion` → human approves → controller re-calls with the token.

## 6. Reuse map (from the projects folder)

| Need | Reuse from |
|------|-----------|
| Cert → stable device/agent ID, in-memory cert mint | **weaver** `internal/identity`, `tlsutil` |
| connect/gRPC service scaffolding, payload signing | **weaver** `lib/grpc`, `lib/signature` |
| Cross-OS service install | **daemon** (`kardianos/service` + cobra) |
| Sealed/gated exec, allow-list, destructive posture | **agentbox** (`--allowed-tools`, read-only seed) |
| Registry / fleet + plugin pattern | **instances-manager**, corral provider/registry + `Doctor` |
| Scaffold + house conventions | **mantle** via `substrate` |

## 7. Roadmap

- **P0 Scaffold** — substrate monorepo (`rexec-agentd` daemon + `rexec` CLI), Taskfile, CI, golangci v2, BSD-3.
- **P1 PKI + enrollment** — CA mint, join token, `Enroll` RPC, `talosconfig`-style credential.
- **P2 mTLS transport + interceptors** — TLS 1.3 client-cert, authenticate/authorize chain.
- **P3 Exec + streaming** — `Exec`/`Info`/`Identity`, live `ExecChunk` stream.
- **P4 Destructive gate** — role table, agent `policy.yaml`, `NeedsApproval` live-approval flow.
- **P5 Cross-OS service** — `kardianos/service` install on mac/linux/windows.
- **P6 Claude Code surface** — `rexec` skills/commands + fleet subagent.
- **P7 (v2) WireGuard overlay** — SideroLink-style mesh + WG-pubkey identity.
