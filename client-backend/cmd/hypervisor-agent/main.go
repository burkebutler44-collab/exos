package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"relay/client-backend/internal/hypervisor"
)

func main() {
	cfg := hypervisor.LoadConfig()
	log.Printf("hypervisor agent starting id=%s grpc=%s libvirt=%s", cfg.HypervisorID, cfg.ControlPlaneGRPCEndpoint, cfg.LibvirtURI)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	executor := hypervisor.OSExecutor{}
	reporter, err := hypervisor.NewGRPCReporter(ctx, cfg)
	if err != nil {
		log.Printf("grpc reporter unavailable, falling back to log reporter: %v", err)
	}
	var activeReporter hypervisor.Reporter = hypervisor.LogReporter{}
	if reporter != nil {
		defer reporter.Close()
		activeReporter = reporter
	}

	agent := &hypervisor.Agent{
		Config:     cfg,
		Hypervisor: hypervisor.NewVirshHypervisor(cfg, executor),
		Reporter:   activeReporter,
		Security:   hypervisor.NewHostSecurityManager(cfg, executor),
		Commands:   hypervisor.NewFileCommandStateStore(cfg.CommandStateDirectory),
	}
	if err := agent.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("run hypervisor agent: %v", err)
	}
}
