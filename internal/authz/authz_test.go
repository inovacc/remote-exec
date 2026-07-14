package authz_test

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/inovacc/remote-exec/internal/authz"
)

func certWithOrg(orgs ...string) *x509.Certificate {
	return &x509.Certificate{Subject: pkix.Name{Organization: orgs}}
}

func TestRoleFromCert_PicksHighest(t *testing.T) {
	require.Equal(t, authz.RoleReader, authz.RoleFromCert(certWithOrg("rex:reader")))
	require.Equal(t, authz.RoleAdmin, authz.RoleFromCert(certWithOrg("rex:reader", "rex:admin")))
	require.Equal(t, authz.RolePublic, authz.RoleFromCert(certWithOrg("unrelated")))
	require.Equal(t, authz.RolePublic, authz.RoleFromCert(certWithOrg()))
}

func TestAllowed(t *testing.T) {
	require.True(t, authz.Allowed(authz.RoleAdmin, authz.RoleOperator))
	require.True(t, authz.Allowed(authz.RoleOperator, authz.RoleOperator))
	require.True(t, authz.Allowed(authz.RoleReader, authz.RolePublic))
	require.False(t, authz.Allowed(authz.RoleReader, authz.RoleOperator))
	require.False(t, authz.Allowed(authz.RoleReader, authz.RoleAdmin))
	require.False(t, authz.Allowed(authz.RolePublic, authz.RoleReader), "no cert cannot meet a role requirement")
}
