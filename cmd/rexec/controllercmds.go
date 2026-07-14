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

// controllerCommands returns the controller-side subcommands Claude Code drives.
func controllerCommands() []*cobra.Command {
	return []*cobra.Command{enrollCmd(), idCmd(), runCmd()}
}

func runCmd() *cobra.Command {
	var configPath, endpoint, workDir string
	var envKV []string
	cmd := &cobra.Command{
		Use:   "run <command> [args...]",
		Short: "Run a command on the enrolled agent, streaming output live",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := clientconfig.Load(configPath)
			if err != nil {
				return err
			}
			if endpoint == "" {
				if len(cfg.Endpoints) == 0 {
					return fmt.Errorf("no endpoint in %s; pass --endpoint", configPath)
				}
				endpoint = cfg.Endpoints[0]
			}
			env, err := parseEnv(envKV)
			if err != nil {
				return err
			}
			conn, err := transport.Dial(cfg, endpoint)
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			stream, err := rexecv1.NewAgentClient(conn).Exec(cmd.Context(), &rexecv1.ExecRequest{
				Command:    args[0],
				Args:       args[1:],
				WorkingDir: workDir,
				Env:        env,
			})
			if err != nil {
				return err
			}
			exitCode := 0
			for {
				chunk, recvErr := stream.Recv()
				if errors.Is(recvErr, io.EOF) {
					break
				}
				if recvErr != nil {
					return recvErr
				}
				switch m := chunk.GetMsg().(type) {
				case *rexecv1.ExecChunk_Stdout:
					_, _ = os.Stdout.Write(m.Stdout)
				case *rexecv1.ExecChunk_Stderr:
					_, _ = os.Stderr.Write(m.Stderr)
				case *rexecv1.ExecChunk_ExitCode:
					exitCode = int(m.ExitCode)
				}
			}
			if exitCode != 0 {
				return fmt.Errorf("remote command exited with code %d", exitCode)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", defaultControllerConfig(), "credential path")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "override the agent endpoint")
	cmd.Flags().StringVar(&workDir, "dir", "", "remote working directory")
	cmd.Flags().StringArrayVar(&envKV, "env", nil, "environment variable KEY=VALUE (repeatable)")
	return cmd
}

func parseEnv(kv []string) (map[string]string, error) {
	if len(kv) == 0 {
		return nil, nil
	}
	env := make(map[string]string, len(kv))
	for _, pair := range kv {
		k, v, ok := strings.Cut(pair, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --env %q, want KEY=VALUE", pair)
		}
		env[k] = v
	}
	return env, nil
}

func enrollCmd() *cobra.Command {
	var tokenVal, commonName, configPath string
	cmd := &cobra.Command{
		Use:   "enroll <endpoint>",
		Short: "Enroll with an agent using a join token and save the credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint := args[0]
			if commonName == "" {
				host, _ := os.Hostname()
				commonName = "rexec@" + host
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()

			cfg, err := transport.Enroll(ctx, endpoint, tokenVal, commonName, nil)
			if err != nil {
				return err
			}
			if err := cfg.Save(configPath); err != nil {
				return err
			}
			cmd.Printf("enrolled with agent %s\n  fingerprint: %s\n  credential:  %s\n",
				cfg.AgentID, cfg.Fingerprint, configPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&tokenVal, "token", "", "single-use join token (required)")
	cmd.Flags().StringVar(&commonName, "cn", "", "controller common name (default rexec@<host>)")
	cmd.Flags().StringVar(&configPath, "config", defaultControllerConfig(), "credential path to write")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

func idCmd() *cobra.Command {
	var configPath, endpoint string
	cmd := &cobra.Command{
		Use:   "id",
		Short: "Ask the enrolled agent for its identity and re-assert the pin",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := clientconfig.Load(configPath)
			if err != nil {
				return err
			}
			if endpoint == "" {
				if len(cfg.Endpoints) == 0 {
					return fmt.Errorf("no endpoint in %s; pass --endpoint", configPath)
				}
				endpoint = cfg.Endpoints[0]
			}
			conn, err := transport.Dial(cfg, endpoint)
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
	cmd.Flags().StringVar(&configPath, "config", defaultControllerConfig(), "credential path")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "override the agent endpoint")
	return cmd
}
