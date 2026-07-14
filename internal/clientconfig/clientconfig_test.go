package clientconfig_test

import (
	"crypto/tls"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/inovacc/remote-exec/internal/clientconfig"
	"github.com/inovacc/remote-exec/internal/pki"
)

// realCredential mints a CA and a signed client cert+key, so the config holds
// genuinely loadable TLS material.
func realCredential(t *testing.T) *clientconfig.Config {
	t.Helper()
	ca, err := pki.NewCA("agent-ca", pki.DefaultCAValidity)
	require.NoError(t, err)
	csrPEM, keyPEM, err := pki.NewCSR("controller-a")
	require.NoError(t, err)
	certPEM, err := ca.Sign(pki.SignRequest{
		CSRPEM:   csrPEM,
		Roles:    []string{"rex:admin"},
		Validity: pki.DefaultLeafValidity,
		Client:   true,
	})
	require.NoError(t, err)
	return &clientconfig.Config{
		CA:          string(ca.CertPEM()),
		ClientCert:  string(certPEM),
		ClientKey:   string(keyPEM),
		Endpoints:   []string{"127.0.0.1:50000"},
		AgentID:     "agent-123",
		Fingerprint: "deadbeef",
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	cfg := realCredential(t)
	path := filepath.Join(t.TempDir(), "sub", "config.yaml")

	require.NoError(t, cfg.Save(path))
	require.FileExists(t, path)

	loaded, err := clientconfig.Load(path)
	require.NoError(t, err)
	require.Equal(t, cfg, loaded)
}

func TestClientTLS_BuildsMutualConfig(t *testing.T) {
	cfg := realCredential(t)

	tlsCfg, err := cfg.ClientTLS()
	require.NoError(t, err)
	require.Len(t, tlsCfg.Certificates, 1)
	require.NotNil(t, tlsCfg.RootCAs)
	require.Equal(t, uint16(tls.VersionTLS13), tlsCfg.MinVersion)
}

func TestClientTLS_IncompleteIsError(t *testing.T) {
	_, err := (&clientconfig.Config{}).ClientTLS()
	require.ErrorIs(t, err, clientconfig.ErrIncomplete)
}
