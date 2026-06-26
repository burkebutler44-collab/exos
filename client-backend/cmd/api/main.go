package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net"
	"os"

	relayv1 "relay/client-backend/gen/go/relay/v1"
	"relay/client-backend/internal/config"
	"relay/client-backend/internal/grpcapi"
	httpapi "relay/client-backend/internal/http"
	"relay/client-backend/internal/http/handlers"
	"relay/client-backend/internal/http/middleware"
	"relay/client-backend/internal/provisioning/central"
	"relay/client-backend/internal/services"
	"relay/client-backend/internal/store"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	repo, err := store.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer repo.Close()

	svc := services.New(repo)
	provision := services.NewProvisionService()
	handlerOptions := []handlers.HandlerOption{}

	if cfg.NATSURL != "" {
		natsBus, err := central.NewNATSBus(cfg.NATSURL, cfg.NATSCredentials, central.WithCommandStreamName(cfg.NATSCommandStream))
		if err != nil {
			log.Printf("connect nats: %v", err)
		} else {
			defer natsBus.Close()
			handlerOptions = append(handlerOptions, handlers.WithProvisionCommandPublisher(natsBus))
		}
	}
	if cfg.NATSMonitoringURL != "" {
		handlerOptions = append(handlerOptions, handlers.WithNATSConnectionMonitor(handlers.NewHTTPNATSConnectionMonitor(cfg.NATSMonitoringURL)))
	}

	grpcOptions, err := grpcServerOptions(cfg)
	if err != nil {
		log.Fatalf("configure grpc tls: %v", err)
	}
	grpcServer := grpc.NewServer(grpcOptions...)
	relayv1.RegisterProvisionerServiceServer(grpcServer, grpcapi.NewProvisionerServer(provision))
	relayv1.RegisterHypervisorServiceServer(grpcServer, grpcapi.NewHypervisorServer(
		grpcapi.NewHypervisorRegistry(repo),
		grpcapi.WithHypervisorAgentToken(cfg.HypervisorAgentToken),
		grpcapi.WithHypervisorClientCertificateAuth(cfg.HypervisorGRPCClientCAFile != ""),
	))

	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("listen grpc: %v", err)
	}
	go func() {
		log.Printf("grpc listening on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("serve grpc: %v", err)
		}
	}()

	router := httpapi.NewRouterWithOptions(svc, handlerOptions, middleware.AuthConfig{
		Issuer:                  cfg.Auth0Issuer,
		Audience:                cfg.Auth0Audience,
		ClientID:                cfg.Auth0ClientID,
		JWKSURL:                 cfg.Auth0JWKSURL,
		AllowInsecureDevHeaders: cfg.AllowInsecureDevAuthHeaders,
	})
	log.Printf("http listening on %s", cfg.HTTPAddr)
	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("serve http: %v", err)
	}
}

func grpcServerOptions(cfg config.Config) ([]grpc.ServerOption, error) {
	if cfg.HypervisorGRPCTLSCertFile == "" || cfg.HypervisorGRPCTLSKeyFile == "" || cfg.HypervisorGRPCClientCAFile == "" {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(cfg.HypervisorGRPCTLSCertFile, cfg.HypervisorGRPCTLSKeyFile)
	if err != nil {
		return nil, err
	}
	caPEM, err := os.ReadFile(cfg.HypervisorGRPCClientCAFile)
	if err != nil {
		return nil, err
	}
	clientCAs := x509.NewCertPool()
	if ok := clientCAs.AppendCertsFromPEM(caPEM); !ok {
		log.Printf("warning: no client CA certificates loaded from %s", cfg.HypervisorGRPCClientCAFile)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	return []grpc.ServerOption{grpc.Creds(credentials.NewTLS(tlsConfig))}, nil
}
