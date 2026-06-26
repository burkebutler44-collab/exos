package central

import (
	"os"
	"time"
)

type Config struct {
	NATSURL                       string
	NATSCredentials               string
	CommandStreamName             string
	EventStreamName               string
	CommandTimeout                time.Duration
	RequestReplyTimeout           time.Duration
	HeartbeatOfflineThreshold     time.Duration
	ProvisioningCommandExpiration time.Duration
}

func LoadConfig() Config {
	return Config{
		NATSURL:                       env("NATS_URL", "nats://localhost:4222"),
		NATSCredentials:               os.Getenv("NATS_CREDENTIALS"),
		CommandStreamName:             env("NATS_COMMAND_STREAM", "RACK_COMMANDS"),
		EventStreamName:               env("NATS_EVENT_STREAM", "RACK_EVENTS"),
		CommandTimeout:                durationEnv("RACK_COMMAND_TIMEOUT", 10*time.Second),
		RequestReplyTimeout:           durationEnv("RACK_REQUEST_TIMEOUT", 3*time.Second),
		HeartbeatOfflineThreshold:     durationEnv("RACK_HEARTBEAT_OFFLINE_THRESHOLD", 90*time.Second),
		ProvisioningCommandExpiration: durationEnv("PROVISIONING_COMMAND_EXPIRATION", 30*time.Minute),
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
