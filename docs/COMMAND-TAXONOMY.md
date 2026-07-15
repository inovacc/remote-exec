# Command Taxonomy — rules contract
<!-- rev:001 -->

Standing rules for the `rexec` (controller) and `rexec-agentd` (agent) CLIs and the
`rexec.v1.Agent` gRPC surface. New commands/methods MUST conform. Rationale + the current
old→new migration live in `docs/superpowers/specs/2026-07-15-command-taxonomy-*`.

## 1. Path shape — always noun → verb

- Every command is `<binary> <noun> <verb>` (deep grouping). **No bare verbs at the root**, with a
  single documented exception: a daemon's primary run action may be a root verb (`rexec-agentd serve`).
- Verbs are actions (`enroll`, `run`, `init`, `install`); nouns are the thing acted on
  (`agent`, `exec`, `ca`, `token`, `service`).

## 2. Casing

- Lowercase **kebab-case** for every command and flag name. Compound verbs keep their hyphen
  (`set-env`). No run-together or snake_case identifiers in the user-facing surface.

## 3. Grouping (sub-noun) threshold

- Group verbs under a sub-noun whenever a domain has ≥2 related verbs or a self-evident cluster.
  Both binaries are deep-grouped: `rexec {agent,exec} <verb>`, `rexec-agentd {ca,token,service} <verb>`.
- A domain's verbs live under exactly one noun — never split the same concept across parents.

## 4. Flags — one canonical name per concept, declared once

- **Persistent flags live on the root** (or the nearest shared parent) and are inherited — never
  re-declared per subcommand. Controller root: `--endpoint`, `--config`. Agent root:
  `--data-dir`, `--listen`.
- One spelling per concept across the whole surface. No synonyms. A flag name means the same
  thing everywhere it appears.
- A target address is always the `--endpoint` **flag** — never a positional on some commands and a
  flag on others.
- Never reuse a name the runtime framework already owns. The mantle `--env` selects the runtime
  environment; a remote process variable is `--set-env KEY=VALUE` (distinct concept, distinct name).

## 5. CLI ↔ gRPC 1:1 alignment

- Each controller leaf that invokes an RPC shares the RPC's canonical verb. The `rexec.v1.Agent`
  method name equals the CLI leaf verb (PascalCase ↔ kebab):

  | CLI leaf | gRPC method |
  |----------|-------------|
  | `rexec agent enroll` | `Agent.Enroll` |
  | `rexec agent identity` | `Agent.Identity` |
  | `rexec agent info` | `Agent.Info` |
  | `rexec exec run` | `Agent.Run` |
  | `rexec exec deploy` | `Agent.Deploy` |

- Renaming a leaf renames its RPC (and vice versa) in the same change. Every RPC has a required
  role in `authz.AgentTable`; a method missing from the table is denied by default.

## 6. Migration policy

- This tool is pre-release (no tag, no external installs), so taxonomy changes are a **clean break**:
  rename in place, no aliases. Each breaking rename ships a `docs/MIGRATION-*.md`. Once the tool has
  a released version and users, revert to the repo's standard deprecation policy (aliases + ≥30-day
  removal) for further taxonomy changes.

## 7. Verification (every taxonomy change)

- `go build ./...` + `go vet ./...` + `task test` green; `task proto` re-run if the proto changed.
- Every RPC present in `authz.AgentTable`; grep proves zero old command paths / flag names remain.
- CLI leaf count == gRPC method count for the aligned set (renamed, never silently dropped).
