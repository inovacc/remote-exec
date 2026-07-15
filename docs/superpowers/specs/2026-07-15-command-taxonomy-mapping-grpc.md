# Command Taxonomy Mapping — gRPC (old → new)

Date: 2026-07-15 · Surface: `rexec.v1.Agent` (`proto/rexec/v1/agent.proto`). Clean break.

The gRPC surface is the wire protocol behind the controller. Only the leaves the CLI invokes are
held 1:1 with the CLI verb; bootstrap/system methods keep their names.

| Old method | New method | CLI leaf | Source | Changed? | Needs-decision |
|------------|-----------|----------|--------|----------|----------------|
| `Agent.Enroll` | `Agent.Enroll` | `rexec agent enroll` | `proto/rexec/v1/agent.proto` | NO | already aligned (bootstrap) |
| `Agent.Identity` | `Agent.Identity` | `rexec agent identity` | same | NO | CLI renamed to match (D1) |
| `Agent.Info` | `Agent.Info` | `rexec agent info` (NEW) | same | NO | CLI leaf added (D2) |
| `Agent.Exec` | `Agent.Run` | `rexec exec run` | same | **YES** | rename to match CLI verb |
| `Agent.Deploy` | `Agent.Deploy` | `rexec exec deploy` | same | NO | already aligned |

## Consequences of the one rename (`Exec` → `Run`)

| Site | Change |
|------|--------|
| `proto/rexec/v1/agent.proto` | `rpc Exec(...)` → `rpc Run(...)`; regenerate with `task proto` |
| `internal/proto/rexec/v1/*.pb.go` | regenerated (do not hand-edit) |
| `internal/agentserver/agentserver.go` | `func (s *Server) Exec(...)` → `Run(...)` |
| `internal/authz/authz.go` `AgentTable` | key `/rexec.v1.Agent/Exec` → `/rexec.v1.Agent/Run` (role stays `rex:operator`) |
| `internal/transport/transport_test.go` | `client.Exec(...)` → `client.Run(...)` in the streaming tests |
| `cmd/rexec/controllercmds.go` | `runCmd` calls `NewAgentClient(conn).Exec(...)` → `.Run(...)` |

`Deploy`'s request type is currently `ExecRequest`; it stays `ExecRequest` (shared with `Run`) —
no message rename needed. `ApprovalRequest`/`ExecChunk` unchanged.

## Count parity check

Before: 5 methods (Enroll, Identity, Info, Exec, Deploy). After: 5 methods (Enroll, Identity, Info,
**Run**, Deploy). One renamed, zero dropped. CLI aligned leaves: 5 (`agent enroll`, `agent identity`,
`agent info`, `exec run`, `exec deploy`) — 1:1 with the 5 methods.

## Not aligned (intentional)

The `ca`/`token`/`service`/`serve` agent-daemon commands have no gRPC counterpart — they are local
admin/lifecycle operations, not remote RPCs. They are excluded from CLI↔gRPC alignment by design.
