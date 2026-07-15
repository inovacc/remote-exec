# Command Taxonomy Redesign — Design Spec

Date: 2026-07-15 · Status: Proposed (awaiting approval) · Scope: full cleanup · Migration: clean break · Surfaces: CLI + gRPC · Shape: deep grouping

Grounded in `docs/command-taxonomy-audit.md`. Rules: `docs/COMMAND-TAXONOMY.md`. Exhaustive
old→new tables: `docs/superpowers/specs/2026-07-15-command-taxonomy-mapping-{cli,grpc}.md`.

## Goals

Resolve every audited issue via a strict noun→verb tree, unified flags, and CLI↔gRPC 1:1 names —
as a clean break (no aliases), since the tool is pre-release.

Audited issues addressed:
- (MED, class 8) endpoint positional-vs-flag → always `--endpoint`.
- (MED, class 3) `--env` overload → remote var is `--set-env`.
- (LOW, class 1) `--data-dir`/`--listen` declared 4× → persistent on the agent root.
- (LOW, class 10) mixed verb ordering / bare verbs → deep noun→verb everywhere.

## Target command tree

### Controller — `rexec`

```
rexec agent enroll <name>   --endpoint --token --cn        (was: rexec enroll <endpoint> --token --cn)
rexec agent identity                                       (was: rexec id)
rexec agent info                                           (NEW — exposes Agent.Info, previously unreachable from CLI)
rexec exec run    <cmd…>    --workdir --set-env            (was: rexec run    <cmd…> --dir --env)
rexec exec deploy <cmd…>    --workdir --set-env --approval --yes   (was: rexec deploy <cmd…> --dir --env --approval --yes)
```
Persistent (root) flags: `--endpoint <host:port>` (required for `agent enroll`; otherwise read from
the credential, override optional), `--config <path>` (credential; default `~/.rexec/config.yaml`).

### Agent daemon — `rexec-agentd`

```
rexec-agentd serve                                         (was: bare `rexec-agentd` root serving — now an explicit primary verb)
rexec-agentd ca init             --force                   (unchanged path)
rexec-agentd token new           --role --ttl              (unchanged path)
rexec-agentd service install|uninstall|start|stop|status   (unchanged group)
rexec-agentd service run                                   (internal: OS-manager entrypoint)
```
Persistent (root) flags: `--data-dir <path>`, `--listen <addr>` — inherited by `serve`, `service`,
`ca`, `token` (removes the 4× duplication).

### gRPC — `rexec.v1.Agent`

Only one rename: `Exec` → `Run` (to match `rexec exec run`). `Enroll`, `Identity`, `Info`, `Deploy`
already align. `authz.AgentTable` key `/rexec.v1.Agent/Exec` → `/rexec.v1.Agent/Run`.

## Key decisions (call out at review)

1. **`id` → `identity`.** Chosen for strict CLI↔gRPC alignment with `Agent.Identity`. Trade-off:
   `identity` is more to type than `id`. Alternative: rename the RPC `Identity`→`Id` instead
   (keeps the short CLI verb) — rejected because `Id` is a poor gRPC method name. **Confirm.**
2. **New `rexec agent info`.** The `Agent.Info` RPC exists but had no CLI verb. Full cleanup exposes
   it. **Confirm** (or leave `Info` gRPC-only and drop this leaf).
3. **`serve` as a root verb.** The daemon's primary action stays a single root verb (documented
   exception to noun→verb) rather than inventing a `server` noun. `service run` remains the
   OS-manager entrypoint. **Confirm** the two aren't merged.
4. **`--set-env` name.** Alternatives: `-e`/`--env-var`. Picked `--set-env` for clarity. **Confirm.**

## Out of scope

`ca`/`token`/`service` verb sets are unchanged (already noun→verb, no drift). No new functionality;
this is a rename/regroup + one proto method rename only.

## Execution (after approval — Phase 6, one concern per PR)

1. **PR-1 gRPC align:** rename `Exec`→`Run` in `proto/rexec/v1/agent.proto`, `task proto`, update
   `agentserver`, `authz.AgentTable`, transport tests. Build+test green.
2. **PR-2 controller regroup:** `agent`/`exec` groups; `identity`, `info`, `run`, `deploy` leaves;
   `--endpoint`/`--config` persistent; `--dir`→`--workdir`, `--env`→`--set-env`. Update
   `.claude/commands/remote/*`, README, CLAUDE-INTEGRATION.
2. **PR-3 agent regroup:** explicit `serve`; `--data-dir`/`--listen` persistent on root; drop the
   per-command re-declarations.
4. **PR-4 docs sweep:** `docs/MIGRATION-2026-07-15-command-taxonomy.md` from the mapping tables;
   grep proves zero old paths/flags remain; refresh `--help` references.

Each PR leaves the tree buildable and tests green.
