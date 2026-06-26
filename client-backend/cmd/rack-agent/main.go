package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"relay/client-backend/internal/provisioning/rackagent"
	"relay/client-backend/internal/provisioning/rackagent/adapters"
	"relay/client-backend/internal/provisioning/rackagent/storage"
)

func main() {
	cfg := rackagent.LoadConfig()
	log.Printf("rack agent starting location=%s rack=%s agent=%s nats=%s", cfg.Location, cfg.RackID, cfg.AgentID, cfg.NATSURL)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conn, err := rackagent.ConnectNATS(cfg.NATSURL, cfg.NATSCredentials)
	if err != nil {
		log.Fatalf("connect nats: %v", err)
	}
	defer conn.Close()

	localStore := storage.NewMemoryStore()
	secrets := adapters.LocalSecretProvider{}
	applier := adapters.NewKubectlApplier(cfg.KubectlBinary)

	var tinkerbell adapters.TinkerbellAdapter
	if cfg.TinkerbellAdapterMode == "mock" {
		tinkerbell = &adapters.MockTinkerbellAdapter{}
	} else {
		kubernetesTinkerbell := adapters.NewKubernetesTinkerbellAdapter(applier, secrets, cfg.KubernetesNamespace)
		kubernetesTinkerbell.ArtifactServer = cfg.ArtifactServer
		kubernetesTinkerbell.InstallPassword = cfg.InstallPassword
		tinkerbell = kubernetesTinkerbell
	}

	var ipmi adapters.IPMIAdapter
	if cfg.IPMIAdapterMode == "kubernetes" {
		ipmi = adapters.NewKubernetesBMCAdapter(applier, cfg.KubernetesNamespace)
	} else {
		ipmi = &adapters.MockIPMIAdapter{}
	}

	agent := &rackagent.Agent{
		RackID:         cfg.RackID,
		Location:       cfg.Location,
		AgentID:        cfg.AgentID,
		Version:        "0.1.0",
		CommandStream:  cfg.CommandStreamName,
		CommandDurable: cfg.CommandConsumerPrefix + "-" + cfg.Location,
		CommandAckWait: cfg.CommandAckWait,
		Storage:        localStore,
		Processed:      localStore,
		Tinkerbell:     tinkerbell,
		IPMI:           ipmi,
		Network:        &adapters.MockNetworkAdapter{},
		Secrets:        secrets,
		Publisher:      rackagent.NewNATSPublisher(conn, cfg.Location),
		HeartbeatEvery: cfg.HeartbeatInterval,
	}

	if err := rackagent.RunNATS(ctx, agent, conn); err != nil && ctx.Err() == nil {
		log.Fatalf("run rack agent: %v", err)
	}
}
