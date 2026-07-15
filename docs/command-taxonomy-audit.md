# Command Taxonomy Audit — remote-exec CLIs

Date: 2026-07-15

Scope: the two cobra binaries in this repo, `rexec` (controller CLI) and
`rexec-agentd` (agent daemon), plus the global/persistent flags injected by
`github.com/inovacc/mantle/bootstrap`. Source is authoritative; every entry
carries a `file:line` reference. This is a point-in-time record — no rev tag.

---

## Command Tree

### `rexec-agentd` (agent daemon)

Root declared at `cmd/rexec-agentd/main.go:22` (`Use: "rexec-agentd"`).
Root persistent flags (`cmd/rexec-agentd/main.go:32-34`):
- `--data-dir` (string, default `defaultDataDir()`, "agent data directory") — `main.go:33`
- `--listen` (string, default `127.0.0.1:50000`, "mTLS gRPC listen address") — `main.go:34`

Subcommands registered via `agentCommands()` (`cmd/rexec-agentd/agentcmds.go:44-46`):

- **`ca`** — "Manage the agent certificate authority" — `agentcmds.go:48-54` (no local flags on the group)
  - **`init`** — "Mint the agent CA and server certificate" — `agentcmds.go:55-95`
    - `--data-dir` (string, default `defaultDataDir()`, "agent data directory") — `agentcmds.go:96`
    - `--force` (bool, default `false`, "overwrite an existing CA") — `agentcmds.go:97`

- **`token`** — "Manage single-use enrollment join tokens" — `agentcmds.go:105-108`
  - **`new`** — "Issue a short-lived, single-use join token" — `agentcmds.go:109-125`
    - `--data-dir` (string, default `defaultDataDir()`, "agent data directory") — `agentcmds.go:126`
    - `--role` (string, default `rex:reader`, "role granted to the enrolling controller") — `agentcmds.go:127`
    - `--ttl` (duration, default `10m`, "token lifetime") — `agentcmds.go:128`

- **`service`** — "Manage rexec-agentd as an OS service (mac/linux/windows)" — `cmd/rexec-agentd/servicecmds.go:55-62`
  - Persistent flags on the group (`servicecmds.go:61-62`):
    - `--data-dir` (string, default `defaultDataDir()`, "agent data directory") — `servicecmds.go:61`
    - `--listen` (string, default `127.0.0.1:50000`, "mTLS gRPC listen address") — `servicecmds.go:62`
  - **`install`** — "install the rexec-agentd service" — `servicecmds.go:79` (via `action`, `servicecmds.go:64-76`)
  - **`uninstall`** — "uninstall the rexec-agentd service" — `servicecmds.go:80`
  - **`start`** — "start the rexec-agentd service" — `servicecmds.go:81`
  - **`stop`** — "stop the rexec-agentd service" — `servicecmds.go:82`
  - **`status`** — "Report the service status" — `servicecmds.go:85-100`
  - **`run`** — "Run under the service manager (invoked by the OS; blocks)" — `servicecmds.go:102-112`

Counts: 3 top-level commands (`ca`, `token`, `service`); 8 leaf commands
(`ca init`, `token new`, `service install|uninstall|start|stop|status|run`).

### `rexec` (controller CLI)

Root declared at `cmd/rexec/main.go:21` (`Use: "rexec"`). The root itself has a
`RunE` (`main.go:38-46`) that just logs a greeting and shuts down (placeholder).
Subcommands registered via `controllerCommands()` (`cmd/rexec/controllercmds.go:56-58`):

- **`enroll <endpoint>`** — "Enroll with an agent using a join token and save the credential" — `controllercmds.go:205-237` (`Args: ExactArgs(1)`)
  - `--token` (string, default `""`, "single-use join token (required)"; `MarkFlagRequired`) — `controllercmds.go:232,235`
  - `--cn` (string, default `""`, "controller common name (default rexec@<host>)") — `controllercmds.go:233`
  - `--config` (string, default `defaultControllerConfig()`, "credential path to write") — `controllercmds.go:234`

- **`id`** — "Ask the enrolled agent for its identity and re-assert the pin" — `controllercmds.go:239-278`
  - `--config` (string, default `defaultControllerConfig()`, "credential path") — `controllercmds.go:275`
  - `--endpoint` (string, default `""`, "override the agent endpoint") — `controllercmds.go:276`

- **`run <command> [args...]`** — "Run a command on the enrolled agent, streaming output live" — `controllercmds.go:60-112` (`Args: MinimumNArgs(1)`)
  - `--config` (string, default `defaultControllerConfig()`, "credential path") — `controllercmds.go:107`
  - `--endpoint` (string, default `""`, "override the agent endpoint") — `controllercmds.go:108`
  - `--dir` (string, default `""`, "remote working directory") — `controllercmds.go:109`
  - `--env` (stringArray, repeatable, "environment variable KEY=VALUE") — `controllercmds.go:110`

- **`deploy <command> [args...]`** — "Run a DESTRUCTIVE command on the agent (admin role + agent policy gate)" — `controllercmds.go:114-188` (`Args: MinimumNArgs(1)`)
  - `--config` (string, default `defaultControllerConfig()`, "credential path") — `controllercmds.go:181`
  - `--endpoint` (string, default `""`, "override the agent endpoint") — `controllercmds.go:182`
  - `--dir` (string, default `""`, "remote working directory") — `controllercmds.go:183`
  - `--env` (stringArray, repeatable, "environment variable KEY=VALUE") — `controllercmds.go:184`
  - `--approval` (string, default `""`, "approval id from a prior APPROVAL_REQUIRED response") — `controllercmds.go:185`
  - `--yes` (bool, default `false`, "auto-approve if the agent policy asks") — `controllercmds.go:186`

Counts: 4 top-level commands (`enroll`, `id`, `run`, `deploy`); all 4 are leaf
commands (no subcommands).

---

## Flag Inventory — inherited globals (bootstrap)

`github.com/inovacc/mantle/bootstrap` wires a common global flag set onto BOTH
roots — `bootstrap.Serve(...)` for `rexec-agentd` (`cmd/rexec-agentd/main.go:56-61`)
and `bootstrap.Configure(...)` for `rexec` (`cmd/rexec/main.go:28-31`). These are
inherited by every subcommand and are NOT per-command drift; noting them once:

`--daemon`, `--env`, `--log-format`, `--log-level`, `--log-source`, `--no-redact`,
`--otel`, `--otel-endpoint`, `--otel-protocol`, `-q/--quiet`, `-v/--verbose`.

Caveat: the agent's own controller-facing `--env` on `rexec run`/`deploy`
(`controllercmds.go:110,184`, KEY=VALUE for the remote process) collides by NAME
with the bootstrap global `--env` (runtime environment selector). See Issue 3.

---

## Diagnosed Issues

### 1. Duplicate/overlapping trees — severity: LOW
No two full hierarchies duplicate each other, but `--data-dir` and `--listen` are
re-declared in three places instead of relying on the root persistent flags:
- Root persistent: `--data-dir` `main.go:33`, `--listen` `main.go:34`.
- `service` persistent (re-declared): `--data-dir` `servicecmds.go:61`, `--listen` `servicecmds.go:62`.
- `ca init` local `--data-dir` `agentcmds.go:96`; `token new` local `--data-dir` `agentcmds.go:126`.

The four local `--data-dir` declarations each bind their OWN variable with an
independent `defaultDataDir()` default, so they shadow the root persistent flag
rather than share it. Same meaning, four bindings — real (if low-impact)
redundancy. `--listen` is duplicated on `service` but is never consumed by
`ca`/`token`, so at least it is harmless there.

### 2. Inconsistent casing/hyphenation — severity: NONE
Flag names are uniformly kebab/lowercase (`--data-dir`, `--log-format`) or short
single words (`--cn`, `--ttl`, `--dir`, `--yes`). Command names are all lowercase
single tokens. No snake_case or run-together offenders found.

### 3. Overloaded verbs / overloaded flag names — severity: MED
- `--env` is overloaded: bootstrap global "runtime environment" selector vs the
  controller's per-command "KEY=VALUE remote process env" on `run`
  (`controllercmds.go:110`) and `deploy` (`controllercmds.go:184`). Same flag
  spelling, two unrelated meanings on the same command — the most concrete
  overload in the CLI.
- `run` verb appears twice with different meanings across binaries: `rexec run`
  = execute a remote command (`controllercmds.go:60`), `rexec-agentd service run`
  = block under the OS service manager (`servicecmds.go:102`). Different trees,
  so mostly acceptable, but worth noting.

### 4. Verb proliferation (extract/list/info scattered) — severity: NONE
No `list`/`info`/`get`/`show` sprawl. The only near-miss is `id` (controller) vs
`status` (service) both being "report" verbs, but they live in different binaries
and read differently. No divergent-flag proliferation found.

### 5. Wide/flat trees — severity: NONE
Largest sibling group is `service` with 6 leaves (`servicecmds.go:78-114`) — a
cohesive, conventional service-control set, well under the ~10-sibling threshold.
Everything else is 1-4 wide.

### 6. Scattered related commands — severity: LOW (intentional)
Enrollment is split across the two binaries: the agent issues the join token
(`rexec-agentd token new`, `agentcmds.go:109`) and the controller consumes it to
enroll (`rexec enroll`, `controllercmds.go:205`). This is the deliberate
daemon/client split (token minted server-side, redeemed client-side), not
accidental drift. Flagged only so the cross-binary relationship is documented.

### 7. Dead/commented-out command bindings — severity: NONE
Every constructed command is registered: `agentCommands()` (`agentcmds.go:45`) and
`controllerCommands()` (`controllercmds.go:57`) each return exactly the commands
they build, all added via `AddCommand`. No commented-out or orphaned
`&cobra.Command{}` literals. The root `rexec` `RunE` is a placeholder
(`main.go:43` `// TODO(app)`) but that is a stub body, not a dead binding.

### 8. Flag-name drift (same meaning, different names) — severity: MED
- Credential/state location is named `--config` on the controller
  (`controllercmds.go:107,181,234,275`) but `--data-dir` on the agent
  (`agentcmds.go:96,126`, `main.go:33`). Both point at the on-disk PKI/credential
  store for their side; the split names make the two CLIs read inconsistently.
  (Arguably justified: agent holds a directory, controller holds a single YAML
  file — different shapes.)
- Endpoint drift: `rexec enroll` takes the endpoint as a POSITIONAL arg
  (`enroll <endpoint>`, `controllercmds.go:208,212`), whereas `id`/`run`/`deploy`
  take it as the `--endpoint` FLAG (`controllercmds.go:108,182,276`). Same concept
  (which agent to talk to), two different surfaces within the same binary — the
  clearest drift here.

### 9. Unclear single-op / orphan commands — severity: LOW
- `ca` and `token` are single-child groups (`ca init`, `token new`) —
  `agentcmds.go:98,129`. The noun-group wrapper anticipates future verbs but today
  adds a layer over a lone leaf. Reasonable for forward extensibility; noted.
- The root `rexec` command has a `RunE` placeholder (`main.go:38-46`) that logs a
  greeting and exits — running `rexec` with no subcommand does effectively nothing
  useful yet. Mild orphan/unclear behavior.

### 10. Mixed verb-noun vs noun-verb ordering — severity: LOW
The CLI mixes two shapes:
- Noun-verb groups: `ca init` (`agentcmds.go:52,56`), `token new`
  (`agentcmds.go:106,110`), `service install|...|run` (`servicecmds.go:58,79`).
- Bare-verb leaves at root: `run`, `deploy`, `enroll`, `id`
  (`controllercmds.go:57`) on the controller.

Within each binary the choice is internally consistent (agent = noun-verb groups,
controller = flat verbs), so this is a mild cross-binary stylistic inconsistency
rather than a defect. `id` is a bare noun used as a verb ("get id"), the one
odd-one-out among the controller's otherwise-imperative leaves.
