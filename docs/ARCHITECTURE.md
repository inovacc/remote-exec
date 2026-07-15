# Architecture
<!-- rev:002 -->

Diagrams reflect the delivered code (v1). See `docs/DESIGN.md` for rationale and
`docs/research/TALOS-SECURE-COMMS.md` for the security derivation.

## System overview

```mermaid
flowchart TB
  subgraph CC["Claude Code (controller side)"]
    cmds["/remote:enroll · id · run · deploy"]
    agent["remote-runner subagent (one per host)"]
    cli["rexec CLI"]
    cmds --> cli
    agent --> cli
  end

  subgraph CTRL["rexec (controller)"]
    cli --> tc["internal/transport (Dial, Enroll)"]
    cli --> cfg["internal/clientconfig (~/.rexec/config.yaml)"]
  end

  subgraph AGENT["rexec-agentd (remote host: mac/linux/windows)"]
    svc["kardianos/service lifecycle"]
    grpc["gRPC server (TLS 1.3, VerifyClientCertIfGiven)"]
    authz["internal/authz interceptors (role from cert O=)"]
    as["internal/agentserver (Agent API)"]
    pki["internal/pki (CA, sign, fingerprint)"]
    enr["internal/enroll (signs client CSR only)"]
    pol["internal/policy (deny|allow|ask + grants)"]
    exe["internal/execute (streaming runner)"]
    svc --> grpc --> authz --> as
    as --> enr --> pki
    as --> pol
    as --> exe
  end

  tc -- "mTLS gRPC :50000" --> grpc
  cfg -. "pinned agentID + fingerprint" .- as
```

## Enrollment (token-bootstrapped, trustd pattern)

```mermaid
sequenceDiagram
  participant Op as Operator
  participant A as rexec-agentd
  participant C as rexec (controller)

  Op->>A: rexec-agentd token new --role rex:operator
  A-->>Op: single-use join token (file-backed, TTL)
  Op->>C: token (out of band)
  C->>C: generate Ed25519 key + CSR
  C->>A: Enroll(token, csr)  [bootstrap TLS, InsecureSkipVerify]
  A->>A: token.Consume (single-use) → role
  A->>A: CA signs CLIENT cert only (O = role)
  A-->>C: {clientCert, caPEM, agentID, serverFingerprint}
  C->>C: save ~/.rexec/config.yaml, pin agentID→fingerprint
```

## Destructive deploy — the approval gate

```mermaid
sequenceDiagram
  participant C as rexec exec deploy
  participant I as authz interceptor
  participant D as agentserver.Deploy
  participant P as policy
  participant H as Human (AskUserQuestion)

  C->>I: Deploy(cmd)  [mTLS, client cert O=rex:admin]
  I->>I: role ≥ rex:admin? 
  alt not admin
    I-->>C: PermissionDenied (need rex:admin)
  else admin
    I->>D: forward
    D->>P: Evaluate(cmd)
    alt policy deny
      D-->>C: PermissionDenied (policy)
    else policy allow
      D->>C: stream stdout/stderr, exit_code
    else policy ask
      D->>D: grants.Issue(cmd) → approval_id
      D-->>C: ExecChunk.needs_approval{operation, approval_id}
      C->>H: AskUserQuestion (Approve/Deny)
      alt Approve
        C->>D: Deploy(cmd, approval_id)
        D->>P: grants.Consume(id, cmd)  [single-use]
        D->>C: stream stdout/stderr, exit_code
      else Deny
        C->>C: declined; approval_id expires unused
      end
    end
  end
```

## Service lifecycle (cross-OS)

```mermaid
sequenceDiagram
  participant M as OS service manager
  participant P as program (kardianos)
  participant S as serveAgent

  M->>P: Start()
  P->>S: go serveAgent(ctx)  [load CA/server cert/policy, listen mTLS]
  Note over S: serves until ctx cancelled
  M->>P: Stop()
  P->>S: cancel ctx → GracefulStop
  P-->>M: done
```
