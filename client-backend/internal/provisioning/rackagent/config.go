package rackagent

import (
	"os"
	"time"
)

type Config struct {
	RackID                string
	Location              string
	AgentID               string
	NATSURL               string
	NATSCredentials       string
	CommandStreamName     string
	CommandConsumerPrefix string
	CommandAckWait        time.Duration
	LocalDBPath           string
	HeartbeatInterval     time.Duration
	TinkerbellEndpoint    string
	TinkerbellAdapterMode string
	IPMIAdapterMode       string
	NetworkAdapterMode    string
	KubernetesNamespace   string
	KubectlBinary         string
	ArtifactServer        string
	InstallPassword       string
	AllowedSubjectPrefix  string
}

func LoadConfig() Config {
	rackID := env("RACK_ID", "rack-dev")
	location := env("LOCATION", env("DC_LOCATION", "ny"))
	return Config{
		RackID:                rackID,
		Location:              location,
		AgentID:               env("AGENT_ID", "agent-dev"),
		NATSURL:               env("NATS_URL", "nats://localhost:4222"),
		NATSCredentials:       os.Getenv("NATS_CREDENTIALS"),
		CommandStreamName:     env("NATS_COMMAND_STREAM", "RACK_COMMANDS"),
		CommandConsumerPrefix: env("NATS_COMMAND_CONSUMER_PREFIX", "rack-agent"),
		CommandAckWait:        durationEnv("NATS_COMMAND_ACK_WAIT", 10*time.Minute),
		LocalDBPath:           env("RACK_AGENT_DB", "rack-agent.db"),
		HeartbeatInterval:     durationEnv("HEARTBEAT_INTERVAL", 15*time.Second),
		TinkerbellEndpoint:    env("TINKERBELL_ENDPOINT", "http://tinkerbell.example.local"),
		TinkerbellAdapterMode: env("TINKERBELL_ADAPTER_MODE", "kubernetes"),
		IPMIAdapterMode:       env("IPMI_ADAPTER_MODE", "mock"),
		NetworkAdapterMode:    env("NETWORK_ADAPTER_MODE", "mock"),
		KubernetesNamespace:   env("TINKERBELL_NAMESPACE", "tink"),
		KubectlBinary:         env("KUBECTL_BINARY", "kubectl"),
		ArtifactServer:        env("TINKERBELL_ARTIFACT_SERVER", ""),
		InstallPassword:       env("TINKERBELL_INSTALL_PASSWORD", ""),
		AllowedSubjectPrefix:  "dc." + location + ".",
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
