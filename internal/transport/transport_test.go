package transport_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/inovacc/remote-exec/internal/agentserver"
	"github.com/inovacc/remote-exec/internal/authz"
	"github.com/inovacc/remote-exec/internal/clientconfig"
	"github.com/inovacc/remote-exec/internal/enroll"
	"github.com/inovacc/remote-exec/internal/pki"
	"github.com/inovacc/remote-exec/internal/policy"
	rexecv1 "github.com/inovacc/remote-exec/internal/proto/rexec/v1"
	"github.com/inovacc/remote-exec/internal/token"
	"github.com/inovacc/remote-exec/internal/transport"
)

const serverSAN = "bufnet"

type harness struct {
	lis           *bufconn.Listener
	tokens        *token.FileStore
	serverCertPEM []byte
}

func mintServerCert(t *testing.T, ca *pki.CA) (certPEM, keyPEM []byte) {
	t.Helper()
	csrPEM, keyPEM, err := pki.NewCSR("rexec-agentd")
	require.NoError(t, err)
	certPEM, err = ca.Sign(pki.SignRequest{
		CSRPEM:   csrPEM,
		Validity: pki.DefaultLeafValidity,
		DNSNames: []string{serverSAN},
	})
	require.NoError(t, err)
	return certPEM, keyPEM
}

func startAgent(t *testing.T, table authz.Table) *harness {
	return startAgentPolicy(t, table, policy.Policy{Destructive: policy.ModeAllow})
}

func startAgentPolicy(t *testing.T, table authz.Table, pol policy.Policy) *harness {
	t.Helper()
	ca, err := pki.NewCA("agent-ca", pki.DefaultCAValidity)
	require.NoError(t, err)
	serverCertPEM, serverKeyPEM := mintServerCert(t, ca)

	tokens := token.NewFileStore(filepath.Join(t.TempDir(), "tokens.json"))
	fp, err := pki.FingerprintPEM(serverCertPEM)
	require.NoError(t, err)
	svc := enroll.NewService(ca, serverCertPEM, "agent-xyz", tokens, pki.DefaultLeafValidity)
	agent := agentserver.New(svc, "agent-xyz", fp, "testhost", "v-test", pol, policy.NewGrants())

	creds, err := transport.ServerCreds(ca.CertPEM(), serverCertPEM, serverKeyPEM)
	require.NoError(t, err)
	srv := transport.NewServer(creds, table, agent)

	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return &harness{lis: lis, tokens: tokens, serverCertPEM: serverCertPEM}
}

func (h *harness) contextDialer() grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return h.lis.DialContext(ctx)
	})
}

// bootstrapDialer mimics the pre-enrollment channel over bufconn.
func (h *harness) bootstrapDialer() func(context.Context, string) (*grpc.ClientConn, error) {
	return func(_ context.Context, _ string) (*grpc.ClientConn, error) {
		return grpc.NewClient("passthrough:///"+serverSAN,
			h.contextDialer(),
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // bootstrap
				MinVersion:         tls.VersionTLS13,
			})),
		)
	}
}

func (h *harness) mtlsClient(t *testing.T, cfg *clientconfig.Config) rexecv1.AgentClient {
	t.Helper()
	creds, err := transport.ClientCreds(cfg, serverSAN)
	require.NoError(t, err)
	conn, err := grpc.NewClient("passthrough:///"+serverSAN, h.contextDialer(), grpc.WithTransportCredentials(creds))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return rexecv1.NewAgentClient(conn)
}

// enroll issues a token for role and enrolls, returning the credential.
func (h *harness) enroll(t *testing.T, role string) *clientconfig.Config {
	t.Helper()
	tok, err := h.tokens.Issue(role, time.Hour)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg, err := transport.Enroll(ctx, serverSAN, tok, "controller-a", h.bootstrapDialer())
	require.NoError(t, err)
	return cfg
}

func (h *harness) enrollReader(t *testing.T) *clientconfig.Config {
	return h.enroll(t, authz.RoleReader)
}

// TestHelperProcess is the subprocess the agent execs during streaming tests.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("REXEC_HELPER") != "1" {
		return
	}
	fmt.Fprint(os.Stdout, "ok-out")
	os.Exit(0)
}

func TestEnrollThenIdentity_OverMTLS(t *testing.T) {
	h := startAgent(t, authz.AgentTable)
	cfg := h.enrollReader(t)

	// The bootstrap response carried the CA, a signed client cert, and the pin.
	require.NotEmpty(t, cfg.CA)
	require.NotEmpty(t, cfg.ClientCert)
	wantFP, err := pki.FingerprintPEM(h.serverCertPEM)
	require.NoError(t, err)
	require.Equal(t, wantFP, cfg.Fingerprint)
	require.Equal(t, "agent-xyz", cfg.AgentID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := h.mtlsClient(t, cfg).Identity(ctx, &rexecv1.IdentityRequest{})
	require.NoError(t, err)
	require.Equal(t, "agent-xyz", resp.GetAgentId())
	require.Equal(t, wantFP, resp.GetFingerprint())
}

func TestReaderCert_RefusedPrivilegedMethod(t *testing.T) {
	// Same Identity method, but the table now demands admin.
	strict := authz.Table{
		"/rexec.v1.Agent/Enroll":   authz.RolePublic,
		"/rexec.v1.Agent/Identity": authz.RoleAdmin,
	}
	h := startAgent(t, strict)
	cfg := h.enrollReader(t) // reader-role cert

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := h.mtlsClient(t, cfg).Identity(ctx, &rexecv1.IdentityRequest{})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "a reader cert must be refused an admin method")
}

func TestExec_OperatorStreamsOutputAndExit(t *testing.T) {
	h := startAgent(t, authz.AgentTable)
	cfg := h.enroll(t, authz.RoleOperator)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stream, err := h.mtlsClient(t, cfg).Exec(ctx, &rexecv1.ExecRequest{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess"},
		Env:     map[string]string{"REXEC_HELPER": "1"},
	})
	require.NoError(t, err)

	var out bytes.Buffer
	gotExit, exitCode := false, -1
	for {
		chunk, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		require.NoError(t, recvErr)
		switch m := chunk.GetMsg().(type) {
		case *rexecv1.ExecChunk_Stdout:
			out.Write(m.Stdout)
		case *rexecv1.ExecChunk_ExitCode:
			gotExit, exitCode = true, int(m.ExitCode)
		}
	}
	require.True(t, gotExit, "stream must end with an exit code")
	require.Equal(t, 0, exitCode)
	require.Contains(t, out.String(), "ok-out")
}

func TestExec_ReaderRefused(t *testing.T) {
	h := startAgent(t, authz.AgentTable)
	cfg := h.enroll(t, authz.RoleReader) // operator required for Exec

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := h.mtlsClient(t, cfg).Exec(ctx, &rexecv1.ExecRequest{Command: "echo"})
	require.NoError(t, err)
	_, err = stream.Recv()
	require.Equal(t, codes.PermissionDenied, status.Code(err), "reader must be refused Exec")
}

type deployResult struct {
	stdout   string
	exit     int
	gotExit  bool
	approval *rexecv1.ApprovalRequest
}

func deploy(t *testing.T, client rexecv1.AgentClient, approvalID string) (deployResult, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stream, err := client.Deploy(ctx, &rexecv1.ExecRequest{
		Command:    os.Args[0],
		Args:       []string{"-test.run=TestHelperProcess"},
		Env:        map[string]string{"REXEC_HELPER": "1"},
		ApprovalId: approvalID,
	})
	require.NoError(t, err)

	var res deployResult
	var out bytes.Buffer
	for {
		chunk, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return res, recvErr
		}
		switch m := chunk.GetMsg().(type) {
		case *rexecv1.ExecChunk_Stdout:
			out.Write(m.Stdout)
		case *rexecv1.ExecChunk_ExitCode:
			res.gotExit, res.exit = true, int(m.ExitCode)
		case *rexecv1.ExecChunk_NeedsApproval:
			res.approval = m.NeedsApproval
		}
	}
	res.stdout = out.String()
	return res, nil
}

func TestDeploy_PolicyAllow_Runs(t *testing.T) {
	h := startAgentPolicy(t, authz.AgentTable, policy.Policy{Destructive: policy.ModeAllow})
	client := h.mtlsClient(t, h.enroll(t, authz.RoleAdmin))

	res, err := deploy(t, client, "")
	require.NoError(t, err)
	require.True(t, res.gotExit)
	require.Equal(t, 0, res.exit)
	require.Contains(t, res.stdout, "ok-out")
	require.Nil(t, res.approval)
}

func TestDeploy_PolicyDeny_Refused(t *testing.T) {
	h := startAgentPolicy(t, authz.AgentTable, policy.Policy{Destructive: policy.ModeDeny})
	client := h.mtlsClient(t, h.enroll(t, authz.RoleAdmin))

	_, err := deploy(t, client, "")
	require.Equal(t, codes.PermissionDenied, status.Code(err), "admin still blocked by agent policy")
}

func TestDeploy_PolicyAsk_ApprovalRoundTrip(t *testing.T) {
	h := startAgentPolicy(t, authz.AgentTable, policy.Policy{Destructive: policy.ModeAsk})
	client := h.mtlsClient(t, h.enroll(t, authz.RoleAdmin))

	// First call: policy says "ask" — the agent must NOT run, only request approval.
	res1, err := deploy(t, client, "")
	require.NoError(t, err)
	require.NotNil(t, res1.approval)
	require.NotEmpty(t, res1.approval.GetApprovalId())
	require.False(t, res1.gotExit, "must not execute before approval")
	require.NotContains(t, res1.stdout, "ok-out")

	// Second call: with the approval id, it runs.
	res2, err := deploy(t, client, res1.approval.GetApprovalId())
	require.NoError(t, err)
	require.True(t, res2.gotExit)
	require.Equal(t, 0, res2.exit)
	require.Contains(t, res2.stdout, "ok-out")

	// Reusing the approval id is rejected (single-use).
	_, err = deploy(t, client, res1.approval.GetApprovalId())
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestNoClientCert_RefusedProtectedMethod(t *testing.T) {
	h := startAgent(t, authz.AgentTable)

	// Dial with the bootstrap (no client cert) channel, then call a protected method.
	conn, err := h.bootstrapDialer()(context.Background(), serverSAN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = rexecv1.NewAgentClient(conn).Identity(ctx, &rexecv1.IdentityRequest{})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
