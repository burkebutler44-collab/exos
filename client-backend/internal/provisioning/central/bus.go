package central

import (
	"context"
	"time"

	"relay/client-backend/internal/provisioning/messages"
)

type CommandPublisher interface {
	PublishCommand(ctx context.Context, subject string, envelope messages.Envelope) error
}

type RequestClient interface {
	Request(ctx context.Context, subject string, envelope messages.Envelope, timeout time.Duration) (messages.Envelope, error)
}

type Metrics interface {
	Inc(name string, labels map[string]string)
	Gauge(name string, value float64, labels map[string]string)
}

type NoopMetrics struct{}

func (NoopMetrics) Inc(name string, labels map[string]string)                  {}
func (NoopMetrics) Gauge(name string, value float64, labels map[string]string) {}
