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

	// Standalone admin commands (CA init, token issuance) that run without the
	// serving runtime. The gRPC Enroll/Exec surface arrives in P2.
	root.AddCommand(agentCommands()...)

	core := func(ctx context.Context, rt *bootstrap.Runtime) error {
		plat, err := platform.New(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = plat.Close(ctx) }()
		rt.Logger.InfoContext(ctx, "rexec-agentd started")
		// Run until the runtime context is cancelled (signal or service stop).
		<-ctx.Done()

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
