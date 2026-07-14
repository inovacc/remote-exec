// Package clientconfig models the talosconfig-style controller credential — the
// agent CA, the controller's client cert+key, target endpoints, and the pinned
// agent identity — persisted to ~/.rexec/config.yaml and turned into an mTLS
// dialing config.
package clientconfig

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrIncomplete is returned when the credential lacks CA/cert/key material.
var ErrIncomplete = errors.New("clientconfig: incomplete credential")

// Config is a single portable controller credential (mirrors talosconfig).
type Config struct {
	CA          string   `yaml:"ca"`           // agent CA certificate PEM
	ClientCert  string   `yaml:"client_cert"`  // controller client certificate PEM
	ClientKey   string   `yaml:"client_key"`   // controller client key PEM
	Endpoints   []string `yaml:"endpoints"`    // agent gRPC endpoints
	AgentID     string   `yaml:"agent_id"`     // pinned agent identity
	Fingerprint string   `yaml:"fingerprint"`  // pinned agent server-cert fingerprint
}

// Load reads a credential from path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("clientconfig: read: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("clientconfig: decode: %w", err)
	}
	return &c, nil
}

// Save writes the credential to path with 0600 permissions.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("clientconfig: encode: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("clientconfig: mkdir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("clientconfig: write: %w", err)
	}
	return nil
}

// ClientTLS builds the mTLS client config: it presents the controller's
// certificate and trusts only the enrolled agent's CA.
func (c *Config) ClientTLS() (*tls.Config, error) {
	if c.CA == "" || c.ClientCert == "" || c.ClientKey == "" {
		return nil, ErrIncomplete
	}
	cert, err := tls.X509KeyPair([]byte(c.ClientCert), []byte(c.ClientKey))
	if err != nil {
		return nil, fmt.Errorf("clientconfig: key pair: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(c.CA)) {
		return nil, errors.New("clientconfig: invalid CA PEM")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      roots,
		MinVersion:   tls.VersionTLS13,
	}, nil
}
