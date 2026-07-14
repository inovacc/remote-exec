// Package pki mints and manages the remote-exec agent certificate authority: an
// Ed25519 CA that signs short-lived client and server leaf certificates from
// CSRs, plus helpers to generate CSRs and fingerprint certificates.
//
// The trust model mirrors Talos: one long-lived CA per agent signs short-lived
// leaves. The agent signs controller CLIENT certificates during enrollment and
// its own SERVER certificate; it never signs another CA.
package pki

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"
)

const (
	blockCert = "CERTIFICATE"
	blockKey  = "PRIVATE KEY"
	blockCSR  = "CERTIFICATE REQUEST"

	// DefaultCAValidity is the lifetime of a freshly minted agent CA (~10y).
	DefaultCAValidity = 10 * 365 * 24 * time.Hour
	// DefaultLeafValidity is the default lifetime of an issued leaf (~24h).
	DefaultLeafValidity = 24 * time.Hour
)

// ErrInvalidPEM is returned when PEM input is empty or undecodable.
var ErrInvalidPEM = errors.New("pki: invalid or empty PEM data")

// CA is an Ed25519 certificate authority.
type CA struct {
	cert *x509.Certificate
	key  ed25519.PrivateKey
}

// NewCA mints a new self-signed Ed25519 CA valid for the given duration.
func NewCA(commonName string, validity time.Duration) (*CA, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("pki: generate CA key: %w", err)
	}
	serial, err := serialNumber()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	if err != nil {
		return nil, fmt.Errorf("pki: create CA cert: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("pki: parse CA cert: %w", err)
	}
	return &CA{cert: cert, key: priv}, nil
}

// LoadCA reconstructs a CA from its certificate and PKCS#8 key PEM.
func LoadCA(certPEM, keyPEM []byte) (*CA, error) {
	cert, err := ParseCert(certPEM)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, ErrInvalidPEM
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pki: parse CA key: %w", err)
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("pki: CA key is not Ed25519")
	}
	return &CA{cert: cert, key: key}, nil
}

// Certificate returns the parsed CA certificate.
func (c *CA) Certificate() *x509.Certificate { return c.cert }

// CertPEM returns the CA certificate in PEM form.
func (c *CA) CertPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: blockCert, Bytes: c.cert.Raw})
}

// KeyPEM returns the CA private key in PKCS#8 PEM form. Sensitive: persist 0600.
func (c *CA) KeyPEM() ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(c.key)
	if err != nil {
		return nil, fmt.Errorf("pki: marshal CA key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: blockKey, Bytes: der}), nil
}

// SignRequest describes a leaf certificate to issue from a CSR.
type SignRequest struct {
	CSRPEM   []byte        // PEM-encoded CSR to sign
	Roles    []string      // encoded into the leaf Subject Organization (O=)
	Validity time.Duration // leaf lifetime
	Client   bool          // true → ClientAuth, false → ServerAuth
	DNSNames []string      // SANs for server certs
	IPs      []net.IP      // SANs for server certs
}

// Sign issues a leaf certificate from the CSR, chaining to this CA. The CSR's
// signature is verified; the requested roles become the leaf's O= field.
func (c *CA) Sign(req SignRequest) ([]byte, error) {
	block, _ := pem.Decode(req.CSRPEM)
	if block == nil {
		return nil, ErrInvalidPEM
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pki: parse CSR: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("pki: CSR signature: %w", err)
	}
	serial, err := serialNumber()
	if err != nil {
		return nil, err
	}
	eku := x509.ExtKeyUsageServerAuth
	if req.Client {
		eku = x509.ExtKeyUsageClientAuth
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   csr.Subject.CommonName,
			Organization: req.Roles,
		},
		NotBefore:   now.Add(-time.Minute),
		NotAfter:    now.Add(req.Validity),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{eku},
		DNSNames:    req.DNSNames,
		IPAddresses: req.IPs,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, csr.PublicKey, c.key)
	if err != nil {
		return nil, fmt.Errorf("pki: sign leaf: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: blockCert, Bytes: der}), nil
}

// NewCSR generates an Ed25519 key and a CSR for the given common name, returning
// both in PEM form. The key never leaves the caller.
func NewCSR(commonName string) (csrPEM, keyPEM []byte, err error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("pki: generate key: %w", err)
	}
	tmpl := &x509.CertificateRequest{Subject: pkix.Name{CommonName: commonName}}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, tmpl, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("pki: create CSR: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("pki: marshal key: %w", err)
	}
	csrPEM = pem.EncodeToMemory(&pem.Block{Type: blockCSR, Bytes: csrDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: blockKey, Bytes: keyDER})
	return csrPEM, keyPEM, nil
}

// ParseCert decodes a single PEM certificate.
func ParseCert(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, ErrInvalidPEM
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pki: parse cert: %w", err)
	}
	return cert, nil
}

// Fingerprint returns the lowercase hex SHA-256 of the certificate DER.
func Fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

// FingerprintPEM parses a PEM certificate and returns its fingerprint.
func FingerprintPEM(certPEM []byte) (string, error) {
	cert, err := ParseCert(certPEM)
	if err != nil {
		return "", err
	}
	return Fingerprint(cert), nil
}

func serialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("pki: serial: %w", err)
	}
	return serial, nil
}
