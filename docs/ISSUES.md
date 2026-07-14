# Known Issues

Limitations observed in the v1 code. Tracked fixes live in `docs/BACKLOG.md` /
`docs/IMPLEMENTATION_TASKS.md`.

| # | Issue | Impact | Workaround / plan |
|---|-------|--------|-------------------|
| 1 | Bootstrap enrollment uses `InsecureSkipVerify` (`internal/transport/transport.go`) | Trust during enroll rests on the single-use token + returned-fingerprint pin; a MITM during the bootstrap TLS handshake is theoretically possible | Issue tokens over a trusted channel; verify the printed fingerprint out-of-band. Fix: SEC-1 (`--expect-fingerprint`) |
| 2 | mantle bootstrap logs to **stdout** for CLI subcommands | Machine parsing must filter JSON log lines; `token new` writes the token to stdout directly to stay parseable | Fix: DX-1 (`--json` output mode) |
| 3 | Approval grants are **in-memory** in the daemon | A daemon restart invalidates outstanding `ask`-mode approval ids | By design (grants are short-lived, 5-min TTL); re-run `deploy` to get a fresh id |
| 4 | Leaf certs do not auto-rotate | Long-running agents will eventually hit leaf expiry | Re-enroll, or implement SEC-2 (rotation daemon) |
| 5 | `service install` requires OS admin privileges and mutates the host | Not exercised in CI | Run manually with elevation; wiring is build-verified |
| 6 | No overlay network | Agents behind NAT are not reachable without port forwarding | v2: NET-1 (WireGuard overlay) |
