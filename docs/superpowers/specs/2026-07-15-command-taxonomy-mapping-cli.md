# Command Taxonomy Mapping — CLI (old → new)

Date: 2026-07-15 · Clean break (no aliases). Exact line refs in `docs/command-taxonomy-audit.md`.

## Controller — `rexec`

| Old | New | Source (func) | Changed? | Needs-decision |
|-----|-----|---------------|----------|----------------|
| `rexec enroll <endpoint> --token --cn --config` | `rexec agent enroll <name> --endpoint --token --cn` | `cmd/rexec/controllercmds.go` `enrollCmd` | YES | endpoint positional→`--endpoint` flag; `<name>` is the client CN (was `--cn`, still a flag) |
| `rexec id --config --endpoint` | `rexec agent identity` | `controllercmds.go` `idCmd` | YES | `id`→`identity` (D1) |
| _(none — `Agent.Info` unreachable from CLI)_ | `rexec agent info` | NEW | YES | expose `Info` (D2) |
| `rexec run <cmd…> --config --endpoint --dir --env` | `rexec exec run <cmd…> --workdir --set-env` | `controllercmds.go` `runCmd` | YES | `--dir`→`--workdir`; `--env`→`--set-env` |
| `rexec deploy <cmd…> --config --endpoint --dir --env --approval --yes` | `rexec exec deploy <cmd…> --workdir --set-env --approval --yes` | `controllercmds.go` `deployCmd` | YES | same flag renames |

Persistent (root) flags — declared once on `rexec`, inherited by all: `--endpoint <host:port>`
(required for `agent enroll`; else from credential), `--config <path>` (default `~/.rexec/config.yaml`).

## Agent daemon — `rexec-agentd`

| Old | New | Source (func) | Changed? | Needs-decision |
|-----|-----|---------------|----------|----------------|
| `rexec-agentd` (bare root serves) | `rexec-agentd serve` | `cmd/rexec-agentd/main.go` `core` | YES | explicit primary verb (D3) |
| `rexec-agentd ca init --data-dir --force` | `rexec-agentd ca init --force` | `agentcmds.go` `caInitCmd` | flags | `--data-dir`→persistent |
| `rexec-agentd token new --data-dir --role --ttl` | `rexec-agentd token new --role --ttl` | `agentcmds.go` `tokenCmd` | flags | `--data-dir`→persistent |
| `rexec-agentd service install\|uninstall\|start\|stop\|status --data-dir --listen` | same paths; `--data-dir`/`--listen` persistent on root | `servicecmds.go` `serviceCmd` | flags | de-duplicate |
| `rexec-agentd service run` | `rexec-agentd service run` | `servicecmds.go` | NO | internal OS-manager entrypoint |

Persistent (root) flags — declared once on `rexec-agentd`: `--data-dir <path>`, `--listen <addr>`.

## Flag unification (drift → canonical)

| Concept | Old spellings | Canonical | Collision resolved |
|---------|---------------|-----------|--------------------|
| Target agent address | positional (`enroll`) + `--endpoint` (`id`/`run`/`deploy`) | `--endpoint` (persistent) | class 8 drift |
| Remote process env var | `--env KEY=VALUE` | `--set-env KEY=VALUE` | class 3 overload vs mantle `--env` |
| Remote working dir | `--dir` | `--workdir` | clarity |
| Agent data dir | `--data-dir` (×4 sites) | `--data-dir` (persistent root) | class 1 duplication |
| Agent listen addr | `--listen` (×2 sites) | `--listen` (persistent root) | class 1 duplication |

## Merge collisions / ambiguity

None. Every old path maps to exactly one new path; no two old names collapse into one.
`--cn` remains a flag (the enroll positional `<name>` is the CN convenience form — confirm at review
whether to keep both or make CN positional-only).

## Downstream references to update (same PRs, not new commands)

- `.claude/commands/remote/{enroll,id,run,deploy}.md` → new paths/flags (and rename files to match).
- `README.md`, `docs/CLAUDE-INTEGRATION.md`, `docs/ARCHITECTURE.md`, `AGENTS.md` command tables.
- `.claude/agents/remote-runner.md` procedure commands.
