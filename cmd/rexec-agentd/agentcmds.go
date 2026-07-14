package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/inovacc/remote-exec/internal/agentserver"
	"github.com/inovacc/remote-exec/internal/authz"
	"github.com/inovacc/remote-exec/internal/enroll"
	"github.com/inovacc/remote-exec/internal/identity"
	"github.com/inovacc/remote-exec/internal/pki"
	"github.com/inovacc/remote-exec/internal/token"
	"github.com/inovacc/remote-exec/internal/transport"
)

// dataLayout resolves the on-disk paths for an agent's PKI + state.
type dataLayout struct{ dir string }

func (d dataLayout) caCert() string     { return filepath.Join(d.dir, "ca.crt") }
func (d dataLayout) caKey() string      { return filepath.Join(d.dir, "ca.key") }
func (d dataLayout) serverCert() string { return filepath.Join(d.dir, "server.crt") }
func (d dataLayout) serverKey() string  { return filepath.Join(d.dir, "server.key") }
func (d dataLayout) agentID() string    { return filepath.Join(d.dir, "agent.id") }
func (d dataLayout) tokens() string     { return filepath.Join(d.dir, "tokens.json") }

func defaultDataDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return "rexec-agentd-data"
	}
	return filepath.Join(base, "rexec-agentd")
}

// agentCommands returns the standalone (non-serving) admin commands.
func agentCommands() []*cobra.Command {
	return []*cobra.Command{caInitCmd(), tokenCmd()}
}

func caInitCmd() *cobra.Command {
	var dir string
	var force bool
	cmd := &cobra.Command{
		Use:   "ca",
		Short: "Manage the agent certificate authority",
	}
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Mint the agent CA and server certificate",
		RunE: func(cmd *cobra.Command, _ []string) error {
			d := dataLayout{dir: dir}
			if _, err := os.Stat(d.caKey()); err == nil && !force {
				return fmt.Errorf("CA already exists at %s (use --force to overwrite)", d.caKey())
			}
			if err := os.MkdirAll(d.dir, 0o700); err != nil {
				return err
			}
			id, err := identity.AgentID(d.agentID())
			if err != nil {
				return err
			}
			ca, err := pki.NewCA("rexec-agentd CA "+id, pki.DefaultCAValidity)
			if err != nil {
				return err
			}
			caKey, err := ca.KeyPEM()
			if err != nil {
				return err
			}
			if err := writeFiles(map[string][]byte{d.caCert(): ca.CertPEM(), d.caKey(): caKey}); err != nil {
				return err
			}
			serverCertPEM, serverKeyPEM, err := mintServerCert(ca)
			if err != nil {
				return err
			}
			if err := writeFiles(map[string][]byte{d.serverCert(): serverCertPEM, d.serverKey(): serverKeyPEM}); err != nil {
				return err
			}
			fp, err := pki.FingerprintPEM(serverCertPEM)
			if err != nil {
				return err
			}
			cmd.Printf("agent CA initialised\n  data dir:    %s\n  agent id:    %s\n  fingerprint: %s\n", d.dir, id, fp)
			return nil
		},
	}
	initCmd.Flags().StringVar(&dir, "data-dir", defaultDataDir(), "agent data directory")
	initCmd.Flags().BoolVar(&force, "force", false, "overwrite an existing CA")
	cmd.AddCommand(initCmd)
	return cmd
}

func tokenCmd() *cobra.Command {
	var dir, role string
	var ttl time.Duration
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage single-use enrollment join tokens",
	}
	newCmd := &cobra.Command{
		Use:   "new",
		Short: "Issue a short-lived, single-use join token",
		RunE: func(cmd *cobra.Command, _ []string) error {
			d := dataLayout{dir: dir}
			store := token.NewFileStore(d.tokens())
			value, err := store.Issue(role, ttl)
			if err != nil {
				return err
			}
			// The token is the machine-readable output: write it to stdout
			// directly so it stays parseable regardless of logger wiring.
			fmt.Fprintln(os.Stdout, value)
			fmt.Fprintf(os.Stderr, "role=%s ttl=%s (single-use)\n", role, ttl)
			return nil
		},
	}
	newCmd.Flags().StringVar(&dir, "data-dir", defaultDataDir(), "agent data directory")
	newCmd.Flags().StringVar(&role, "role", "rex:reader", "role granted to the enrolling controller")
	newCmd.Flags().DurationVar(&ttl, "ttl", 10*time.Minute, "token lifetime")
	cmd.AddCommand(newCmd)
	return cmd
}

// mintServerCert generates the agent's own server key + certificate.
func mintServerCert(ca *pki.CA) (certPEM, keyPEM []byte, err error) {
	csrPEM, keyPEM, err := pki.NewCSR("rexec-agentd")
	if err != nil {
		return nil, nil, err
	}
	host, _ := os.Hostname()
	dns := []string{"localhost"}
	if host != "" {
		dns = append(dns, host)
	}
	certPEM, err = ca.Sign(pki.SignRequest{
		CSRPEM:   csrPEM,
		Validity: 365 * 24 * time.Hour,
		DNSNames: dns,
		IPs:      []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	})
	if err != nil {
		return nil, nil, err
	}
	return certPEM, keyPEM, nil
}

// serveAgent loads the agent's PKI from disk and serves the mTLS gRPC Agent API
// until ctx is cancelled.
func serveAgent(ctx context.Context, logger *slog.Logger, dir, listen, version string) error {
	d := dataLayout{dir: dir}
	caCert, err := os.ReadFile(d.caCert())
	if err != nil {
		return fmt.Errorf("read CA (run `rexec-agentd ca init` first): %w", err)
	}
	caKey, err := os.ReadFile(d.caKey())
	if err != nil {
		return fmt.Errorf("read CA key: %w", err)
	}
	ca, err := pki.LoadCA(caCert, caKey)
	if err != nil {
		return err
	}
	serverCert, err := os.ReadFile(d.serverCert())
	if err != nil {
		return fmt.Errorf("read server cert: %w", err)
	}
	serverKey, err := os.ReadFile(d.serverKey())
	if err != nil {
		return fmt.Errorf("read server key: %w", err)
	}
	id, err := identity.AgentID(d.agentID())
	if err != nil {
		return err
	}
	fp, err := pki.FingerprintPEM(serverCert)
	if err != nil {
		return err
	}

	tokens := token.NewFileStore(d.tokens())
	svc := enroll.NewService(ca, serverCert, id, tokens, pki.DefaultLeafValidity)
	host, _ := os.Hostname()
	agent := agentserver.New(svc, id, fp, host, version)

	creds, err := transport.ServerCreds(caCert, serverCert, serverKey)
	if err != nil {
		return err
	}
	srv := transport.NewServer(creds, authz.AgentTable, agent)

	lis, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listen, err)
	}
	logger.InfoContext(ctx, "rexec-agentd serving mTLS gRPC",
		slog.String("listen", listen), slog.String("agent_id", id), slog.String("fingerprint", fp))

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(lis) }()
	select {
	case <-ctx.Done():
		srv.GracefulStop()
		return nil
	case serveErr := <-errCh:
		return serveErr
	}
}

func writeFiles(files map[string][]byte) error {
	for path, data := range files {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
