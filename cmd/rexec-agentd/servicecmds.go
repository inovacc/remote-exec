package main

import (
	"context"
	"log/slog"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

// program adapts serveAgent to the kardianos/service lifecycle so rexec-agentd
// can run as a launchd / systemd / Windows service across macOS, Linux, Windows.
type program struct {
	dataDir string
	listen  string
	version string
	cancel  context.CancelFunc
	done    chan struct{}
}

func (p *program) Start(_ service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
		if err := serveAgent(ctx, slog.Default(), p.dataDir, p.listen, p.version); err != nil {
			slog.Error("rexec-agentd service exited", slog.String("error", err.Error()))
		}
	}()
	return nil
}

func (p *program) Stop(_ service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.done != nil {
		<-p.done
	}
	return nil
}

func newAgentService(dataDir, listen, version string) (service.Service, error) {
	cfg := &service.Config{
		Name:        "rexec-agentd",
		DisplayName: "rexec-agentd (remote-exec agent)",
		Description: "Secure cross-OS remote execution agent (mTLS gRPC).",
		Arguments:   []string{"service", "run", "--data-dir", dataDir, "--listen", listen},
	}
	return service.New(&program{dataDir: dataDir, listen: listen, version: version}, cfg)
}

// serviceCmd exposes install/uninstall/start/stop/status/run for the OS service.
func serviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage rexec-agentd as an OS service (mac/linux/windows)",
	}

	action := func(name string, fn func(service.Service) error) *cobra.Command {
		return &cobra.Command{
			Use:   name,
			Short: name + " the rexec-agentd service",
			RunE: func(cmd *cobra.Command, _ []string) error {
				svc, err := newAgentService(dataDirOf(cmd), listenOf(cmd), version)
				if err != nil {
					return err
				}
				return fn(svc)
			},
		}
	}

	cmd.AddCommand(
		action("install", func(s service.Service) error { return s.Install() }),
		action("uninstall", func(s service.Service) error { return s.Uninstall() }),
		action("start", func(s service.Service) error { return s.Start() }),
		action("stop", func(s service.Service) error { return s.Stop() }),
	)

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Report the service status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newAgentService(dataDirOf(cmd), listenOf(cmd), version)
			if err != nil {
				return err
			}
			st, err := svc.Status()
			if err != nil {
				return err
			}
			cmd.Println(statusString(st))
			return nil
		},
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run under the service manager (invoked by the OS; blocks)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newAgentService(dataDirOf(cmd), listenOf(cmd), version)
			if err != nil {
				return err
			}
			return svc.Run()
		},
	}

	cmd.AddCommand(statusCmd, runCmd)
	return cmd
}

func statusString(s service.Status) string {
	switch s {
	case service.StatusRunning:
		return "running"
	case service.StatusStopped:
		return "stopped"
	default:
		return "unknown (not installed?)"
	}
}
