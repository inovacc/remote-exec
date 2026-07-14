package transport_test

import (
	"context"
	"crypto/tls"
	"net"
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
	t.Helper()
	ca, err := pki.NewCA("agent-ca", pki.DefaultCAValidity)
	require.NoError(t, err)
	serverCertPEM, serverKeyPEM := mintServerCert(t, ca)

	tokens := token.NewFileStore(filepath.Join(t.TempDir(), "tokens.json"))
	fp, err := pki.FingerprintPEM(serverCertPEM)
	require.NoError(t, err)
	svc := enroll.NewService(ca, serverCertPEM, "agent-xyz", tokens, pki.DefaultLeafValidity)
	agent := agentserver.New(svc, "agent-xyz", fp, "testhost", "v-test")

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

// enrollReader issues a reader token and enrolls, returning the credential.
func (h *harness) enrollReader(t *testing.T) *clientconfig.Config {
	t.Helper()
	tok, err := h.tokens.Issue(authz.RoleReader, time.Hour)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg, err := transport.Enroll(ctx, serverSAN, tok, "controller-a", h.bootstrapDialer())
	require.NoError(t, err)
	return cfg
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
