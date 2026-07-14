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

	// Standalone admin commands: CA init and token issuance.
	root.AddCommand(agentCommands()...)

	var dataDir, listen string
	root.PersistentFlags().StringVar(&dataDir, "data-dir", defaultDataDir(), "agent data directory")
	root.PersistentFlags().StringVar(&listen, "listen", "127.0.0.1:50000", "mTLS gRPC listen address")

	core := func(ctx context.Context, rt *bootstrap.Runtime) error {
		plat, err := platform.New(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = plat.Close(ctx) }()

		rt.Logger.InfoContext(ctx, "rexec-agentd started")
		if serveErr := serveAgent(ctx, rt.Logger, dataDir, listen, version); serveErr != nil {
			return serveErr
		}

		// The runtime context is already cancelled; use a fresh bounded
		// context so observability data still flushes on shutdown.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		rt.Logger.InfoContext(shutdownCtx, "rexec-agentd stopping")
		return rt.Shutdown(shutdownCtx)
	}

	if err := bootstrap.Serve(root, a, core,
		bootstrap.WithAppName("rexec-agentd"),
		bootstrap.WithVersion(version),
		bootstrap.WithConfigPath("config.yaml"),
		bootstrap.WithEnvPrefix("REMOTE_EXEC"),
	); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
