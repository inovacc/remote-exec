// Package transport wires the Agent gRPC service onto mTLS. The server presents
// its certificate and verifies client certificates against the agent CA; the
// authz interceptor then gates each method on the client cert's role. Enrollment
// runs on a bootstrap channel (token-authenticated) before the controller holds
// a certificate.
package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/inovacc/remote-exec/internal/agentserver"
	"github.com/inovacc/remote-exec/internal/authz"
	"github.com/inovacc/remote-exec/internal/clientconfig"
	"github.com/inovacc/remote-exec/internal/pki"
	rexecv1 "github.com/inovacc/remote-exec/internal/proto/rexec/v1"
)

// ServerCreds builds mTLS server credentials: the agent presents serverCert and
// verifies any client cert against the CA. Client certs are verified when
// presented but not required at the TLS layer, so the public Enroll method stays
// reachable on the same port; the authz interceptor enforces cert presence for
// every protected method.
func ServerCreds(caPEM, serverCertPEM, serverKeyPEM []byte) (credentials.TransportCredentials, error) {
	cert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("transport: server key pair: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("transport: invalid CA PEM")
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    roots,
		ClientAuth:   tls.VerifyClientCertIfGiven,
		MinVersion:   tls.VersionTLS13,
	}), nil
}

// ClientCreds builds mTLS client credentials from an enrolled credential. When
// serverName is non-empty it overrides the name verified against the server
// cert SANs (used in tests; production derives it from the dial target).
func ClientCreds(cfg *clientconfig.Config, serverName string) (credentials.TransportCredentials, error) {
	tlsCfg, err := cfg.ClientTLS()
	if err != nil {
		return nil, err
	}
	if serverName != "" {
		tlsCfg.ServerName = serverName
	}
	return credentials.NewTLS(tlsCfg), nil
}

// Dial opens an mTLS connection to endpoint using an enrolled credential. The
// server name is derived from the endpoint host, verified against the agent
// server cert's SANs.
func Dial(cfg *clientconfig.Config, endpoint string) (*grpc.ClientConn, error) {
	creds, err := ClientCreds(cfg, "")
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("transport: dial %s: %w", endpoint, err)
	}
	return conn, nil
}

// bootstrapCreds returns the pre-enrollment client credentials: the controller
// has no certificate and does not yet trust the agent CA, so it presents no
// client cert and skips server verification. Trust rests on the single-use join
// token; the returned agent fingerprint is pinned for all later mTLS calls.
func bootstrapCreds() credentials.TransportCredentials {
	return credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // bootstrap: token-authenticated, fingerprint pinned on return
		MinVersion:         tls.VersionTLS13,
	})
}

// NewServer assembles a gRPC server with the given mTLS credentials, the authz
// table, and the Agent service registered.
func NewServer(creds credentials.TransportCredentials, table authz.Table, agent *agentserver.Server) *grpc.Server {
	srv := grpc.NewServer(
		grpc.Creds(creds),
		grpc.ChainUnaryInterceptor(authz.UnaryInterceptor(table)),
	)
	rexecv1.RegisterAgentServer(srv, agent)
	return srv
}

// Enroll dials endpoint over the bootstrap channel, submits a freshly generated
// CSR with the join token, and returns the resulting controller credential
// (unsaved). dialer lets callers inject an in-process connection in tests; pass
// nil for a normal network dial.
func Enroll(ctx context.Context, endpoint, token, commonName string, dialer func(context.Context, string) (*grpc.ClientConn, error)) (*clientconfig.Config, error) {
	csrPEM, keyPEM, err := pki.NewCSR(commonName)
	if err != nil {
		return nil, err
	}
	conn, err := dialBootstrap(ctx, endpoint, dialer)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	resp, err := rexecv1.NewAgentClient(conn).Enroll(ctx, &rexecv1.EnrollRequest{Token: token, CsrPem: csrPEM})
	if err != nil {
		return nil, fmt.Errorf("transport: enroll: %w", err)
	}
	return &clientconfig.Config{
		CA:          string(resp.GetCaPem()),
		ClientCert:  string(resp.GetClientCertPem()),
		ClientKey:   string(keyPEM),
		Endpoints:   []string{endpoint},
		AgentID:     resp.GetAgentId(),
		Fingerprint: resp.GetFingerprint(),
	}, nil
}

func dialBootstrap(ctx context.Context, endpoint string, dialer func(context.Context, string) (*grpc.ClientConn, error)) (*grpc.ClientConn, error) {
	if dialer != nil {
		return dialer(ctx, endpoint)
	}
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(bootstrapCreds()))
	if err != nil {
		return nil, fmt.Errorf("transport: dial bootstrap: %w", err)
	}
	return conn, nil
}
