package enroll_test

import (
	"crypto/x509"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/inovacc/remote-exec/internal/enroll"
	"github.com/inovacc/remote-exec/internal/pki"
)

type stubConsumer struct {
	role string
	err  error
	seen []string
}

func (s *stubConsumer) Consume(value string) (string, error) {
	s.seen = append(s.seen, value)
	if s.err != nil {
		return "", s.err
	}
	return s.role, nil
}

// buildAgent mints a CA and the agent's own server certificate.
func buildAgent(t *testing.T) (*pki.CA, []byte) {
	t.Helper()
	ca, err := pki.NewCA("agent-ca", pki.DefaultCAValidity)
	require.NoError(t, err)
	csrPEM, _, err := pki.NewCSR("rexec-agentd")
	require.NoError(t, err)
	serverCert, err := ca.Sign(pki.SignRequest{
		CSRPEM:   csrPEM,
		Validity: pki.DefaultLeafValidity,
		DNSNames: []string{"localhost"},
	})
	require.NoError(t, err)
	return ca, serverCert
}

func TestEnroll_SignsClientCertWithTokenRole(t *testing.T) {
	ca, serverCert := buildAgent(t)
	consumer := &stubConsumer{role: "rex:operator"}
	svc := enroll.NewService(ca, serverCert, "agent-123", consumer, pki.DefaultLeafValidity)

	csrPEM, _, err := pki.NewCSR("controller-a")
	require.NoError(t, err)

	res, err := svc.Enroll(csrPEM, "join-token")
	require.NoError(t, err)
	require.Equal(t, "agent-123", res.AgentID)
	require.Equal(t, []string{"join-token"}, consumer.seen)

	// Fingerprint pins the agent's server cert.
	wantFP, err := pki.FingerprintPEM(serverCert)
	require.NoError(t, err)
	require.Equal(t, wantFP, res.Fingerprint)

	// Issued client cert carries the granted role and chains to the CA.
	leaf, err := pki.ParseCert(res.ClientCertPEM)
	require.NoError(t, err)
	require.Equal(t, []string{"rex:operator"}, leaf.Subject.Organization)
	require.Contains(t, leaf.ExtKeyUsage, x509.ExtKeyUsageClientAuth)

	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(res.CAPEM))
	_, err = leaf.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)
}

func TestEnroll_RejectsBadToken(t *testing.T) {
	ca, serverCert := buildAgent(t)
	consumer := &stubConsumer{err: errors.New("boom")}
	svc := enroll.NewService(ca, serverCert, "agent-123", consumer, pki.DefaultLeafValidity)

	csrPEM, _, err := pki.NewCSR("controller-a")
	require.NoError(t, err)

	_, err = svc.Enroll(csrPEM, "bad")
	require.Error(t, err)
}

func TestEnroll_LeafValidityHonored(t *testing.T) {
	ca, serverCert := buildAgent(t)
	svc := enroll.NewService(ca, serverCert, "agent-123", &stubConsumer{role: "rex:reader"}, 2*time.Hour)

	csrPEM, _, err := pki.NewCSR("controller-a")
	require.NoError(t, err)
	res, err := svc.Enroll(csrPEM, "tok")
	require.NoError(t, err)

	leaf, err := pki.ParseCert(res.ClientCertPEM)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(2*time.Hour), leaf.NotAfter, 5*time.Minute)
}
