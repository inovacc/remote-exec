// Package enroll implements the token-bootstrapped controller enrollment flow.
// The agent validates a single-use join token and signs the controller's client
// CSR — and only a client CSR — returning the credential material the controller
// pins. This is the trustd pattern from Talos, minus the cluster machinery.
package enroll

import (
	"fmt"
	"time"

	"github.com/inovacc/remote-exec/internal/pki"
)

// TokenConsumer validates and consumes a single-use join token, returning the
// role it grants. Implemented by token.FileStore.
type TokenConsumer interface {
	Consume(value string) (role string, err error)
}

// Result is the credential material returned to an enrolling controller.
type Result struct {
	ClientCertPEM []byte // the controller's signed client certificate
	CAPEM         []byte // the agent CA, so the controller can verify the agent
	AgentID       string // the agent's stable identity
	Fingerprint   string // SHA-256 of the agent's server cert — the pin target
}

// Service signs controller client certificates during enrollment.
type Service struct {
	ca            *pki.CA
	serverCertPEM []byte
	agentID       string
	tokens        TokenConsumer
	leafValidity  time.Duration
}

// NewService wires an enrollment service. serverCertPEM is the agent's own
// server certificate, whose fingerprint controllers pin.
func NewService(ca *pki.CA, serverCertPEM []byte, agentID string, tokens TokenConsumer, leafValidity time.Duration) *Service {
	return &Service{
		ca:            ca,
		serverCertPEM: serverCertPEM,
		agentID:       agentID,
		tokens:        tokens,
		leafValidity:  leafValidity,
	}
}

// Enroll consumes the join token and signs the client CSR with the role the
// token granted. It never signs a CA or server certificate.
func (s *Service) Enroll(csrPEM []byte, tokenValue string) (*Result, error) {
	role, err := s.tokens.Consume(tokenValue)
	if err != nil {
		return nil, fmt.Errorf("enroll: token: %w", err)
	}
	clientCert, err := s.ca.Sign(pki.SignRequest{
		CSRPEM:   csrPEM,
		Roles:    []string{role},
		Validity: s.leafValidity,
		Client:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("enroll: sign client cert: %w", err)
	}
	fingerprint, err := pki.FingerprintPEM(s.serverCertPEM)
	if err != nil {
		return nil, fmt.Errorf("enroll: fingerprint: %w", err)
	}
	return &Result{
		ClientCertPEM: clientCert,
		CAPEM:         s.ca.CertPEM(),
		AgentID:       s.agentID,
		Fingerprint:   fingerprint,
	}, nil
}
