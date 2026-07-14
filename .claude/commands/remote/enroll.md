---
description: Enroll this controller with a remote rexec agent using a single-use join token
argument-hint: <endpoint> --token <token> [--config <path>]
allowed-tools: Bash(rexec enroll:*)
---
Enroll the controller with a remote agent by running:

`rexec enroll $ARGUMENTS`

The agent signs a client certificate (the role is fixed by the token the operator issued)
and returns the CA plus its identity. Report back:
- the **agent id** and **fingerprint** (this fingerprint is now pinned — every later call
  re-asserts it and errors on mismatch),
- the **credential path** written (default `~/.rexec/config.yaml`).

If enrollment fails with `unknown token`, the token was already used (single-use) or expired —
ask the operator to issue a fresh one with `rexec-agentd token new`.
