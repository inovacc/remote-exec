# AGENTS.md
<!-- rev:002 -->

Canonical cross-tool agent instructions for **remote-exec** (read by Claude Code via
`CLAUDE.md`'s `@AGENTS.md` import, and by Codex/Cursor/Gemini directly).

## What this is

Secure, cross-OS remote execution for Claude Code. Two Go binaries in one module
(`github.com/inovacc/remote-exec`), monorepo layout:

- `cmd/rexec-agentd` — the agent daemon: runs as an OS service, terminates mTLS, serves the
  `rexec.v1.Agent` gRPC API, enforces the destructive-op policy.
- `cmd/rexec` — the controller CLI Claude Code drives: `agent enroll`, `agent identity`, `agent info`, `exec run`, `exec deploy`.

Security model is derived from Talos Linux — see `docs/DESIGN.md` and
`docs/research/TALOS-SECURE-COMMS.md`.

## Build / test / lint (prefer `task`)

| Task | Command |
|------|---------|
| Build | `task build` (→ `go build ./...`) |
| Fast tests | `task test` (→ `go test -short ./...`) |
| Full tests | `task test:full` (→ `go test -race -coverprofile=coverage.out ./...`) |
| Coverage % | `task test:cover` |
| Vet | `task vet` (→ `go vet ./...`) |
| Lint | `task lint` (→ `golangci-lint run ./...`, config `.golangci.yml` v2) |
| Regenerate gRPC | `task proto` (needs `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc`) |
| All checks | `task check` (fix → fmt → vet → lint → test) |

Run programs with `go run ./cmd/<app>` — never `go build && ./app`. `go build ./...` and
`go vet ./...` must stay clean; keep `gofmt` clean.

## Layout

```
cmd/rexec-agentd/   daemon: ca init, token new, service *, serve; agentcmds.go, servicecmds.go
cmd/rexec/          controller CLI: agent {enroll,identity,info}, exec {run,deploy} (controllercmds.go)
internal/pki        Ed25519 CA, CSR sign, fingerprint
internal/identity   stable persisted agent UUID
internal/token      single-use join tokens (file-backed, flock)
internal/enroll     enrollment service (signs client CSR only)
internal/clientconfig  talosconfig-style credential + mTLS dial config
internal/authz      role-from-cert + per-method unary/stream interceptors
internal/transport  gRPC mTLS server/client, bootstrap enroll, Dial
internal/agentserver Agent gRPC service impl (Enroll/Identity/Info/Run/Deploy)
internal/execute    streaming command runner
internal/policy     destructive-op policy + single-use approval grants
internal/proto/rexec/v1  generated gRPC (edit proto/rexec/v1/agent.proto, run `task proto`)
```

## Code style

Idiomatic Go per Uber Go Style Guide + Effective Go. Wrap errors with `%w`; sentinel errors
for comparison. Keep `cmd/` layers thin — logic lives in `internal/`. Package doc comment
(`// Package <name> …`) on every package. Shared code goes in module-root `internal/` (imported
by both binaries); per-binary code under `cmd/<app>/internal/`.

## Security (this project's whole point)

- Never weaken the authz gate: role lives in the client cert Subject `O=`; every gRPC method has
  a required role in `authz.AgentTable`. New RPCs MUST be added to that table (missing = denied).
- The agent signs **client** CSRs only, never CA or server certs elsewhere.
- Join tokens and approval grants are **single-use**; keep them so.
- Destructive commands go through `internal/policy` (deny|allow|ask) + a single-use, 5-min,
  command-bound approval. Never bypass it; `--yes` is opt-in by the human only.
- Never commit `config.yaml` (runtime AppSecret), `*.db`, `*.key`, or agent data — all gitignored.

## Commits / PRs

Conventional commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`). No AI attribution / no
`Co-Authored-By`. Keep the working tree gofmt/vet/build clean before committing.

## Docs

Living instruction/architecture docs carry `<!-- rev:NNN -->` after the H1 (bump on edit).
`docs/DESIGN.md` is the spec; `docs/ROADMAP.md` tracks phases; `docs/CLAUDE-INTEGRATION.md`
covers the Claude Code surface (`.claude/commands/remote/*`, the `remote-runner` subagent).
