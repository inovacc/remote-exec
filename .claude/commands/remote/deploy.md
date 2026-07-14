---
description: Run a DESTRUCTIVE command on a remote agent, honoring the human-in-the-loop approval gate
argument-hint: <command> [args...] [--dir <path>] [--env KEY=VAL]
allowed-tools: Bash(rexec deploy:*), AskUserQuestion
---
Run a destructive command on the enrolled agent through the destructive-op gate:

`rexec deploy $ARGUMENTS`

The agent enforces three checks — admin role (from the client cert), its local `policy.yaml`,
and (when the policy says `ask`) a one-time live approval. Handle the outcome:

1. **It streamed output and exited 0** → report the result. Done.
2. **`PermissionDenied ... policy denies`** → the agent's policy forbids this destructive
   command. Report it; do not attempt a workaround.
3. **`PermissionDenied ... need "rex:admin"`** → this controller isn't an admin; report that a
   `rex:admin` credential is required.
4. **Output contains `APPROVAL_REQUIRED approval_id=<id> operation="<op>" reason="<reason>"`**
   (and exit is non-zero) → the agent is waiting for human approval. You MUST:
   a. Call **AskUserQuestion** asking whether to approve running `<op>` on the remote agent,
      quoting `<reason>`. Options: **Approve** / **Deny**.
   b. If **Approve** → re-run `rexec deploy --approval <id> $ARGUMENTS` and report the streamed
      result.
   c. If **Deny** → stop and report that the destructive operation was declined. The approval
      id expires unused.

Never pass `--yes` (which skips the human approval) unless the user explicitly instructed you to.
