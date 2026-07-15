---
description: Ask an enrolled remote agent for its identity and re-assert the pinned fingerprint
argument-hint: [--credential <path>] [--endpoint <host:port>]
allowed-tools: Bash(rexec agent identity:*)
---
Run `rexec agent identity $ARGUMENTS`.

This dials the agent over mTLS and returns its stable **agent id** and **server-cert
fingerprint**, verifying the fingerprint against the one pinned at enrollment. Report the id
and whether the pin matched (`pin OK`). A `fingerprint mismatch` error means the agent's
identity changed since enrollment (re-provisioned host, or a possible MITM) — surface it as a
security warning and do not proceed with further remote calls until the operator confirms.
