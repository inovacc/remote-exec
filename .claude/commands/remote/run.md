---
description: Run a NON-destructive command (build/test/analyze) on a remote agent, streaming output
argument-hint: <command> [args...] [--dir <path>] [--env KEY=VAL]
allowed-tools: Bash(rexec run:*)
---
Run a non-destructive command on the enrolled agent and stream its output:

`rexec run $ARGUMENTS`

This uses the `Exec` RPC (minimum role `rex:operator`) — for builds, tests, and analysis.
Stdout and stderr stream back live; the command's remote exit code becomes the local exit code.

- If it exits non-zero, report the failing output — do not retry blindly.
- If you get `PermissionDenied ... need "rex:operator"`, this controller enrolled with a
  `rex:reader` credential; it can only call read-only methods (`id`). Ask the operator to
  re-enroll with an operator token.
- For **destructive** actions (deploy, release, delete, infra changes) use `/remote:deploy`
  instead — it goes through the approval gate.
