# Migration — command taxonomy (2026-07-15)

A **clean break** (no aliases) landed on 2026-07-15, deep-grouping both CLIs into
strict noun→verb and aligning the gRPC surface. If you have scripts, credentials, or
habits from before this date, update them per the tables below. Rationale:
`docs/COMMAND-TAXONOMY.md`.

## Controller — `rexec`

| Old | New |
|-----|-----|
| `rexec enroll <endpoint> --token <t>` | `rexec agent enroll --endpoint <endpoint> --token <t>` |
| `rexec id` | `rexec agent identity` |
| _(none)_ | `rexec agent info` (new — host os/arch/version) |
| `rexec run <cmd…>` | `rexec exec run <cmd…>` |
| `rexec deploy <cmd…>` | `rexec exec deploy <cmd…>` |

Flags:

| Old | New | Why |
|-----|-----|-----|
| positional `<endpoint>` on `enroll` | `--endpoint <host:port>` (persistent) | consistent across all controller commands |
| `--config <path>` | `--credential <path>` (persistent) | the mantle runtime already owns `--config` |
| `--dir <path>` | `--workdir <path>` | clarity |
| `--env KEY=VAL` | `--set-env KEY=VAL` | `--env` is the mantle runtime-environment selector |

## Agent daemon — `rexec-agentd`

| Old | New |
|-----|-----|
| `rexec-agentd` (bare — implicitly served) | `rexec-agentd serve` (explicit) |
| `rexec-agentd ca init` | `rexec-agentd ca init` (unchanged) |
| `rexec-agentd token new` | `rexec-agentd token new` (unchanged) |
| `rexec-agentd service …` | `rexec-agentd service …` (unchanged) |

Flags: `--data-dir` and `--listen` are now declared once on the root and inherited by
`serve`, `ca`, `token`, and `service` (behaviour unchanged; just no longer re-declared).

## gRPC — `rexec.v1.Agent`

| Old method | New method |
|------------|-----------|
| `Agent.Exec` | `Agent.Run` |
| `Agent.Enroll` / `Identity` / `Info` / `Deploy` | unchanged |

Regenerate clients from the updated `proto/rexec/v1/agent.proto` (`task proto`).

## Credentials & data

No credential/data-format change — `~/.rexec/config.yaml` and the agent data dir are
unchanged. Only the commands and flags that read them changed. Existing enrolled
credentials keep working with the new commands.

## Claude Code

The `/remote:enroll|id|run|deploy` slash commands keep their names; only the underlying
`rexec …` invocations they run were updated (already done in `.claude/commands/remote/*`).
