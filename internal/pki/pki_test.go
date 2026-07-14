package pki

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewCA_RoundTrip(t *testing.T) {
	ca, err := NewCA("test-ca", DefaultCAValidity)
	require.NoError(t, err)
	require.True(t, ca.Certificate().IsCA)

	keyPEM, err := ca.KeyPEM()
	require.NoError(t, err)

	loaded, err := LoadCA(ca.CertPEM(), keyPEM)
	require.NoError(t, err)
	require.Equal(t, ca.Certificate().Raw, loaded.Certificate().Raw)
}

func TestSign_ClientCert_ChainsAndCarriesRole(t *testing.T) {
	ca, err := NewCA("test-ca", DefaultCAValidity)
	require.NoError(t, err)

	csrPEM, _, err := NewCSR("controller-1")
	require.NoError(t, err)

	certPEM, err := ca.Sign(SignRequest{
		CSRPEM:   csrPEM,
		Roles:    []string{"rex:admin"},
		Validity: DefaultLeafValidity,
		Client:   true,
	})
	require.NoError(t, err)

	leaf, err := ParseCert(certPEM)
	require.NoError(t, err)
	require.Equal(t, []string{"rex:admin"}, leaf.Subject.Organization)
	require.Equal(t, "controller-1", leaf.Subject.CommonName)
	require.Contains(t, leaf.ExtKeyUsage, x509.ExtKeyUsageClientAuth)

	roots := x509.NewCertPool()
	roots.AddCert(ca.Certificate())
	_, err = leaf.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err, "client leaf must chain to the CA")
}

func TestSign_ServerCert_HasServerAuthAndSAN(t *testing.T) {
	ca, err := NewCA("test-ca", DefaultCAValidity)
	require.NoError(t, err)

	csrPEM, _, err := NewCSR("rexec-agentd")
	require.NoError(t, err)

	certPEM, err := ca.Sign(SignRequest{
		CSRPEM:   csrPEM,
		Validity: DefaultLeafValidity,
		Client:   false,
		DNSNames: []string{"localhost"},
	})
	require.NoError(t, err)

	leaf, err := ParseCert(certPEM)
	require.NoError(t, err)
	require.Contains(t, leaf.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	require.Contains(t, leaf.DNSNames, "localhost")
}

func TestSign_RejectsGarbageCSR(t *testing.T) {
	ca, err := NewCA("test-ca", DefaultCAValidity)
	require.NoError(t, err)

	_, err = ca.Sign(SignRequest{CSRPEM: []byte("not a pem"), Validity: time.Hour})
	require.ErrorIs(t, err, ErrInvalidPEM)
}

func TestFingerprint_StableAndHex(t *testing.T) {
	ca, err := NewCA("test-ca", DefaultCAValidity)
	require.NoError(t, err)

	fp, err := FingerprintPEM(ca.CertPEM())
	require.NoError(t, err)
	require.Len(t, fp, 64)
	require.Equal(t, Fingerprint(ca.Certificate()), fp)
}
