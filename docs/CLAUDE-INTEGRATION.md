# Using remote-exec from Claude Code

How a Claude Code session (and a fleet of subagents) drives actions on remote
mac/linux/windows machines over the secure `rexec` channel.

Date: 2026-07-14

## Pieces shipped in `.claude/`

| Path | What it is |
|------|-----------|
| `.claude/commands/remote/enroll.md` | `/remote:enroll --endpoint <host:port> --token <t>` — join an agent, pin its identity |
| `.claude/commands/remote/id.md` | `/remote:id` — ask an agent for its id, re-assert the fingerprint pin |
| `.claude/commands/remote/run.md` | `/remote:run <cmd>` — non-destructive exec (build/test/analyze), streamed |
| `.claude/commands/remote/deploy.md` | `/remote:deploy <cmd>` — destructive exec through the approval gate |
| `.claude/agents/remote-runner.md` | subagent that drives one agent end-to-end (one per target host) |

These are picked up automatically when Claude Code runs inside this repo.

## The identity handshake ("ask the other instance for its id")

1. On the agent host: `rexec-agentd ca init` then `rexec-agentd service install && rexec-agentd service start` (or foreground: `rexec-agentd serve`).
2. Operator issues a scoped, single-use token: `rexec-agentd token new --role rex:operator`.
3. Claude enrolls: `/remote:enroll --endpoint <host:port> --token <t>` → the agent signs a client cert,
   returns its **agent id** + **fingerprint**, which Claude pins.
4. `/remote:id` re-asserts the pin on demand — a mismatch is surfaced as a security warning.

Roles carried in the client cert (`rex:reader ⊂ operator ⊂ admin`) decide, cryptographically,
what the controller may call — a reader cert cannot invoke `exec run`, and only an admin cert can
reach `exec deploy`.

## The destructive-op gate ("ask if it can perform destructive operations")

`/remote:deploy` and the `remote-runner` agent implement human-in-the-loop approval:

```
rexec exec deploy <cmd>
  ├─ agent policy = allow → runs, streams output
  ├─ agent policy = deny  → PermissionDenied (no workaround)
  └─ agent policy = ask   → APPROVAL_REQUIRED approval_id=… operation=… reason=…
                             │
                             ├─ Claude calls AskUserQuestion (Approve / Deny)
                             ├─ Approve → rexec exec deploy --approval <id> <cmd>  (runs, single-use)
                             └─ Deny    → declined; the approval id expires unused
```

The approval id is single-use, command-bound, and expires in 5 minutes. `--yes` skips the
prompt and must only be used when the human explicitly opts out of approval.

## Fleet pattern (cross-OS)

To act across several hosts at once, dispatch one `remote-runner` subagent per target,
each with its own credential/endpoint:

```
main session
 ├─ Task(remote-runner, cfg=~/.rexec/linux-builder.yaml)   → run: go build ./...
 ├─ Task(remote-runner, cfg=~/.rexec/mac-notary.yaml)      → deploy: notarize (approval gate)
 └─ Task(remote-runner, cfg=~/.rexec/win-tester.yaml)      → run: go test ./...
```

Each subagent confirms its target via `rexec agent identity`, does its work, handles any approval
prompt, and returns a compact result. Siblings never touch each other's credentials.
