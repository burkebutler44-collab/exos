package handlers

import (
	"context"
	"encoding/json"

	"relay/client-backend/internal/provisioning/messages"

	nats "github.com/nats-io/nats.go"
)

type ProvisionCommandPublisher interface {
	PublishCommand(ctx context.Context, subject string, envelope messages.Envelope) error
}

type HandlerOption func(*Handler)

func WithProvisionCommandPublisher(publisher ProvisionCommandPublisher) HandlerOption {
	return func(h *Handler) {
		h.provisionPublisher = publisher
	}
}

type NATSCommandPublisher struct {
	conn *nats.Conn
}

func NewNATSCommandPublisher(conn *nats.Conn) *NATSCommandPublisher {
	return &NATSCommandPublisher{conn: conn}
}

func (p *NATSCommandPublisher) PublishCommand(ctx context.Context, subject string, envelope messages.Envelope) error {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- p.conn.Publish(subject, payload) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
