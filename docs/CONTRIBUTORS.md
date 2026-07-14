# Contributors

| Name | GitHub | Role |
|------|--------|------|
| dyammarcano | [@dyammarcano](https://github.com/dyammarcano) | Owner |

## Contributing

**Toolchain:** Go (see the `go` directive in `go.mod`). gRPC codegen needs `protoc` +
`protoc-gen-go` + `protoc-gen-go-grpc`.

**Workflow:**
1. `task check` — runs fix → fmt → vet → lint → test. Keep it green.
2. Add tests for new logic (`internal/*` packages are unit-tested; transport uses bufconn).
3. Editing the API: change `proto/rexec/v1/agent.proto`, run `task proto`, and add any new RPC
   to `authz.AgentTable` (a method missing from the table is denied by default).
4. Conventional commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`). No AI attribution.

**Security invariants (do not regress):** role-from-cert authorization, agent signs client CSRs
only, single-use tokens/approvals, the destructive-op policy gate. See `AGENTS.md` → Security.

**Build/test commands:** see `AGENTS.md`. Never commit `config.yaml`, `*.db`, `*.key`, or agent
data (all gitignored).
