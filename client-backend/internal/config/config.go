package config

import (
	"os"
	"strings"
)

type Config struct {
	DatabaseURL                 string
	HTTPAddr                    string
	GRPCAddr                    string
	Environment                 string
	Auth0Domain                 string
	Auth0ClientID               string
	Auth0Audience               string
	Auth0Issuer                 string
	Auth0JWKSURL                string
	AllowInsecureDevAuthHeaders bool
	NATSURL                     string
	NATSCredentials             string
	NATSCommandStream           string
	NATSMonitoringURL           string
	HypervisorAgentToken        string
	HypervisorGRPCTLSCertFile   string
	HypervisorGRPCTLSKeyFile    string
	HypervisorGRPCClientCAFile  string
}

func Load() Config {
	environment := env("APP_ENV", "development")
	auth0Domain := strings.TrimSpace(env("AUTH0_DOMAIN", ""))
	auth0Issuer := strings.TrimSpace(env("AUTH0_ISSUER", ""))
	if auth0Issuer == "" && auth0Domain != "" {
		auth0Issuer = "https://" + strings.TrimSuffix(strings.TrimPrefix(auth0Domain, "https://"), "/") + "/"
	}

	auth0JWKSURL := strings.TrimSpace(env("AUTH0_JWKS_URL", ""))
	if auth0JWKSURL == "" && auth0Issuer != "" {
		auth0JWKSURL = strings.TrimRight(auth0Issuer, "/") + "/.well-known/jwks.json"
	}

	return Config{
		DatabaseURL:                 env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/relay?sslmode=disable"),
		HTTPAddr:                    env("HTTP_ADDR", ":8080"),
		GRPCAddr:                    env("GRPC_ADDR", ":9090"),
		Environment:                 environment,
		Auth0Domain:                 auth0Domain,
		Auth0ClientID:               env("AUTH0_CLIENT_ID", ""),
		Auth0Audience:               env("AUTH0_AUDIENCE", ""),
		Auth0Issuer:                 auth0Issuer,
		Auth0JWKSURL:                auth0JWKSURL,
		AllowInsecureDevAuthHeaders: envBool("ALLOW_INSECURE_DEV_AUTH_HEADERS", environment != "production"),
		NATSURL:                     env("NATS_URL", "nats://localhost:4222"),
		NATSCredentials:             env("NATS_CREDENTIALS", ""),
		NATSCommandStream:           env("NATS_COMMAND_STREAM", "RACK_COMMANDS"),
		NATSMonitoringURL:           env("NATS_MONITORING_URL", "http://localhost:8222"),
		HypervisorAgentToken:        env("HYPERVISOR_AGENT_TOKEN", ""),
		HypervisorGRPCTLSCertFile:   env("HYPERVISOR_GRPC_TLS_CERT_FILE", ""),
		HypervisorGRPCTLSKeyFile:    env("HYPERVISOR_GRPC_TLS_KEY_FILE", ""),
		HypervisorGRPCClientCAFile:  env("HYPERVISOR_GRPC_CLIENT_CA_FILE", ""),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes"
}
