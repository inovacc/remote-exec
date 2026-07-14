# CLAUDE.md
<!-- rev:001 -->

Claude Code entry point for **remote-exec**. Canonical cross-tool instructions live in
**AGENTS.md** (imported below).

@AGENTS.md

## Claude-Code-only

- **Remote actions:** drive agents through the `.claude/commands/remote/*` slash commands
  (`/remote:enroll`, `/remote:id`, `/remote:run`, `/remote:deploy`) and the `remote-runner`
  subagent (one per target host). Wiring + the fleet pattern: `docs/CLAUDE-INTEGRATION.md`.
- **Destructive ops:** `/remote:deploy` may return `APPROVAL_REQUIRED approval_id=…` — surface it
  via `AskUserQuestion` (Approve/Deny), then re-run with `--approval <id>`. Never pass `--yes`
  unless the human explicitly said to.
- **Regenerating gRPC:** after editing `proto/rexec/v1/agent.proto`, run `task proto` (never hand-
  edit `internal/proto/**`). New RPCs must be added to `authz.AgentTable` or they're denied.
