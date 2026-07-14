package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	return []*cobra.Command{enrollCmd(), idCmd()}
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
