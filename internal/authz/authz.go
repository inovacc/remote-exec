// Package authz enforces the destructive-op gate: it reads the caller's role
// from the client certificate's Subject Organization (O=) and authorizes each
// gRPC method against a per-method minimum-role table. Authorization is bound to
// the cryptographic identity, so a lower-privileged certificate physically
// cannot invoke a higher-privileged method.
package authz

import (
	"context"
	"crypto/x509"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Roles, ordered by increasing privilege.
const (
	RolePublic   = ""             // no certificate required (bootstrap methods)
	RoleReader   = "rex:reader"   // read-only: Identity, Info, non-destructive analyze
	RoleOperator = "rex:operator" // build/test and other non-destructive execution
	RoleAdmin    = "rex:admin"    // deploy, delete, arbitrary destructive operations
)

var rank = map[string]int{RoleReader: 1, RoleOperator: 2, RoleAdmin: 3}

// Table maps a full gRPC method name to the minimum role required to call it.
// A method mapped to RolePublic needs no certificate; a method absent from the
// table is denied by default.
type Table map[string]string

// RoleFromCert returns the highest known role encoded in the certificate's
// Subject Organization, or RolePublic if none is present.
func RoleFromCert(cert *x509.Certificate) string {
	best, bestRank := RolePublic, 0
	for _, org := range cert.Subject.Organization {
		if r, ok := rank[org]; ok && r > bestRank {
			best, bestRank = org, r
		}
	}
	return best
}

// Allowed reports whether a caller holding role `have` may invoke a method whose
// minimum is `need`.
func Allowed(have, need string) bool {
	if need == RolePublic {
		return true
	}
	return rank[have] >= rank[need] && rank[have] > 0
}

// ErrNoClientCert is returned when a protected method is called without a
// verified client certificate.
var ErrNoClientCert = errors.New("authz: no verified client certificate")

// UnaryInterceptor enforces `table` on every unary call. Public methods pass
// without a certificate; all others require a certificate whose role satisfies
// the table.
func UnaryInterceptor(table Table) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		need, ok := table[info.FullMethod]
		if !ok {
			return nil, status.Errorf(codes.PermissionDenied, "method %s not permitted", info.FullMethod)
		}
		if need == RolePublic {
			return handler(ctx, req)
		}
		role, err := roleFromContext(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		if !Allowed(role, need) {
			return nil, status.Errorf(codes.PermissionDenied, "role %q insufficient, need %q", role, need)
		}
		return handler(ctx, req)
	}
}

func roleFromContext(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", ErrNoClientCert
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.PeerCertificates) == 0 {
		return "", ErrNoClientCert
	}
	return RoleFromCert(tlsInfo.State.PeerCertificates[0]), nil
}

// AgentTable is the production authorization table for the Agent service.
var AgentTable = Table{
	"/rexec.v1.Agent/Enroll":   RolePublic,
	"/rexec.v1.Agent/Identity": RoleReader,
	"/rexec.v1.Agent/Info":     RoleReader,
}
