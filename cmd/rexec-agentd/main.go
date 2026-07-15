// Command rexec-agentd is the remote-exec agent daemon: it manages the agent CA
// and join tokens, terminates mTLS, serves the rexec.v1.Agent gRPC API, and
// enforces the destructive-op policy. It runs as an OS service on mac/linux/windows.
package main

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/inovacc/mantle/bootstrap"

	"github.com/inovacc/remote-exec/cmd/rexec-agentd/internal/app"
	"github.com/inovacc/remote-exec/cmd/rexec-agentd/internal/platform"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:   "rexec-agentd",
		Short: "rexec-agentd — remote-exec agent daemon (mTLS gRPC, cross-OS service)",
	}

	a := app.New()

	// --data-dir / --listen are declared once on the root and inherited by every
	// subcommand (serve, ca, token, service).
	root.PersistentFlags().String("data-dir", defaultDataDir(), "agent data directory")
	root.PersistentFlags().String("listen", "127.0.0.1:50000", "mTLS gRPC listen address")

	// Standalone admin commands: CA init, token issuance, OS-service management.
	root.AddCommand(agentCommands()...)

	// serve is the explicit primary action (the daemon's foreground run).
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the mTLS gRPC Agent API in the foreground",
	}
	core := func(ctx context.Context, rt *bootstrap.Runtime) error {
		plat, err := platform.New(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = plat.Close(ctx) }()

		rt.Logger.InfoContext(ctx, "rexec-agentd started")
		if serveErr := serveAgent(ctx, rt.Logger, dataDirOf(serveCmd), listenOf(serveCmd), version); serveErr != nil {
			return serveErr
		}

		// The runtime context is already cancelled; use a fresh bounded
		// context so observability data still flushes on shutdown.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		rt.Logger.InfoContext(shutdownCtx, "rexec-agentd stopping")
		return rt.Shutdown(shutdownCtx)
	}

	if err := bootstrap.Serve(serveCmd, a, core,
		bootstrap.WithAppName("rexec-agentd"),
		bootstrap.WithVersion(version),
		bootstrap.WithConfigPath("config.yaml"),
		bootstrap.WithEnvPrefix("REMOTE_EXEC"),
	); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	root.AddCommand(serveCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
