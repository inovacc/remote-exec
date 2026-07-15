package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/inovacc/remote-exec/internal/clientconfig"
	rexecv1 "github.com/inovacc/remote-exec/internal/proto/rexec/v1"
	"github.com/inovacc/remote-exec/internal/transport"
)

func defaultControllerConfig() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".rexec", "config.yaml")
	}
	return filepath.Join(home, ".rexec", "config.yaml")
}

// controllerCommands returns the deep-grouped controller subcommands
// (`agent <verb>` and `exec <verb>`). The --endpoint and --config flags are
// persistent on the root (see main.go) and inherited here.
func controllerCommands() []*cobra.Command {
	return []*cobra.Command{agentGroup(), execGroup()}
}

// credentialFlag / endpointFlag read the root's inherited persistent flags.
// The credential path uses --credential because the runtime owns --config.
func credentialFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("credential")
	if v == "" {
		return defaultControllerConfig()
	}
	return v
}

func endpointFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("endpoint")
	return v
}

// dialAgent loads the credential, resolves the endpoint (flag overrides the
// pinned one), and opens an mTLS connection.
func dialAgent(cmd *cobra.Command) (*grpc.ClientConn, error) {
	cfg, err := clientconfig.Load(credentialFlag(cmd))
	if err != nil {
		return nil, err
	}
	endpoint := endpointFlag(cmd)
	if endpoint == "" {
		if len(cfg.Endpoints) == 0 {
			return nil, fmt.Errorf("no endpoint in %s; pass --endpoint", credentialFlag(cmd))
		}
		endpoint = cfg.Endpoints[0]
	}
	return transport.Dial(cfg, endpoint)
}

// --- agent group -----------------------------------------------------------

func agentGroup() *cobra.Command {
	g := &cobra.Command{Use: "agent", Short: "Enroll with and query a remote agent"}
	g.AddCommand(agentEnrollCmd(), agentIdentityCmd(), agentInfoCmd())
	return g
}

func agentEnrollCmd() *cobra.Command {
	var token, commonName string
	cmd := &cobra.Command{
		Use:   "enroll",
		Short: "Enroll with an agent using a join token and save the credential",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			endpoint := endpointFlag(cmd)
			if endpoint == "" {
				return errors.New("--endpoint is required to enroll")
			}
			if commonName == "" {
				host, _ := os.Hostname()
				commonName = "rexec@" + host
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()

			cfg, err := transport.Enroll(ctx, endpoint, token, commonName, nil)
			if err != nil {
				return err
			}
			if err := cfg.Save(credentialFlag(cmd)); err != nil {
				return err
			}
			cmd.Printf("enrolled with agent %s\n  fingerprint: %s\n  credential:  %s\n",
				cfg.AgentID, cfg.Fingerprint, credentialFlag(cmd))
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "single-use join token (required)")
	cmd.Flags().StringVar(&commonName, "cn", "", "controller common name (default rexec@<host>)")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

func agentIdentityCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "identity",
		Short: "Ask the enrolled agent for its identity and re-assert the pin",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := clientconfig.Load(credentialFlag(cmd))
			if err != nil {
				return err
			}
			conn, err := dialAgent(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			resp, err := rexecv1.NewAgentClient(conn).Identity(ctx, &rexecv1.IdentityRequest{})
			if err != nil {
				return err
			}
			if cfg.Fingerprint != "" && resp.GetFingerprint() != cfg.Fingerprint {
				return fmt.Errorf("fingerprint mismatch: pinned %s, got %s (possible MITM)",
					cfg.Fingerprint, resp.GetFingerprint())
			}
			cmd.Printf("agent id:    %s\n  fingerprint: %s (pin OK)\n", resp.GetAgentId(), resp.GetFingerprint())
			return nil
		},
	}
}

func agentInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Report the agent host OS, arch, and version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, err := dialAgent(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			resp, err := rexecv1.NewAgentClient(conn).Info(ctx, &rexecv1.InfoRequest{})
			if err != nil {
				return err
			}
			cmd.Printf("os:       %s/%s\n  hostname: %s\n  version:  %s\n",
				resp.GetOs(), resp.GetArch(), resp.GetHostname(), resp.GetVersion())
			return nil
		},
	}
}

// --- exec group ------------------------------------------------------------

func execGroup() *cobra.Command {
	g := &cobra.Command{Use: "exec", Short: "Run commands on a remote agent"}
	g.AddCommand(execRunCmd(), execDeployCmd())
	return g
}

func execRunCmd() *cobra.Command {
	var workDir string
	var envKV []string
	cmd := &cobra.Command{
		Use:   "run <command> [args...]",
		Short: "Run a non-destructive command on the agent, streaming output live",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := parseEnv(envKV)
			if err != nil {
				return err
			}
			conn, err := dialAgent(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			stream, err := rexecv1.NewAgentClient(conn).Run(cmd.Context(), &rexecv1.ExecRequest{
				Command: args[0], Args: args[1:], WorkingDir: workDir, Env: env,
			})
			if err != nil {
				return err
			}
			exitCode, _, err := consumeStream(stream)
			if err != nil {
				return err
			}
			if exitCode != 0 {
				return fmt.Errorf("remote command exited with code %d", exitCode)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&workDir, "workdir", "", "remote working directory")
	cmd.Flags().StringArrayVar(&envKV, "set-env", nil, "remote environment variable KEY=VALUE (repeatable)")
	return cmd
}

func execDeployCmd() *cobra.Command {
	var workDir, approvalID string
	var envKV []string
	var autoYes bool
	cmd := &cobra.Command{
		Use:   "deploy <command> [args...]",
		Short: "Run a DESTRUCTIVE command on the agent (admin role + agent policy gate)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := parseEnv(envKV)
			if err != nil {
				return err
			}
			conn, err := dialAgent(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()
			client := rexecv1.NewAgentClient(conn)

			send := func(id string) (int, *rexecv1.ApprovalRequest, error) {
				stream, sErr := client.Deploy(cmd.Context(), &rexecv1.ExecRequest{
					Command: args[0], Args: args[1:], WorkingDir: workDir, Env: env, ApprovalId: id,
				})
				if sErr != nil {
					return 0, nil, sErr
				}
				return consumeStream(stream)
			}

			exitCode, approval, err := send(approvalID)
			if err != nil {
				return err
			}
			if approval != nil {
				if !autoYes {
					fmt.Fprintf(os.Stdout, "APPROVAL_REQUIRED approval_id=%s operation=%q reason=%q\n",
						approval.GetApprovalId(), approval.GetOperation(), approval.GetReason())
					return fmt.Errorf("destructive operation needs approval; re-run with --approval %s (or --yes)", approval.GetApprovalId())
				}
				cmd.PrintErrf("policy requires approval for %q — auto-approving (--yes)\n", approval.GetOperation())
				exitCode, approval, err = send(approval.GetApprovalId())
				if err != nil {
					return err
				}
				if approval != nil {
					return errors.New("agent still requests approval after --yes; aborting")
				}
			}
			if exitCode != 0 {
				return fmt.Errorf("remote command exited with code %d", exitCode)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&workDir, "workdir", "", "remote working directory")
	cmd.Flags().StringArrayVar(&envKV, "set-env", nil, "remote environment variable KEY=VALUE (repeatable)")
	cmd.Flags().StringVar(&approvalID, "approval", "", "approval id from a prior APPROVAL_REQUIRED response")
	cmd.Flags().BoolVar(&autoYes, "yes", false, "auto-approve if the agent policy asks (interactive/trusted use)")
	return cmd
}

// --- shared helpers --------------------------------------------------------

// consumeStream writes stdout/stderr chunks to the terminal and returns the
// remote exit code, or a pending approval request if the agent gated a
// destructive command with "ask".
func consumeStream(stream grpc.ServerStreamingClient[rexecv1.ExecChunk]) (int, *rexecv1.ApprovalRequest, error) {
	exit := 0
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return exit, nil, nil
		}
		if err != nil {
			return exit, nil, err
		}
		switch m := chunk.GetMsg().(type) {
		case *rexecv1.ExecChunk_Stdout:
			_, _ = os.Stdout.Write(m.Stdout)
		case *rexecv1.ExecChunk_Stderr:
			_, _ = os.Stderr.Write(m.Stderr)
		case *rexecv1.ExecChunk_ExitCode:
			exit = int(m.ExitCode)
		case *rexecv1.ExecChunk_NeedsApproval:
			return exit, m.NeedsApproval, nil
		}
	}
}

func parseEnv(kv []string) (map[string]string, error) {
	if len(kv) == 0 {
		return nil, nil
	}
	env := make(map[string]string, len(kv))
	for _, pair := range kv {
		k, v, ok := strings.Cut(pair, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --set-env %q, want KEY=VALUE", pair)
		}
		env[k] = v
	}
	return env, nil
}
