package central

import (
	"context"
	"encoding/json"
	"time"

	"relay/client-backend/internal/provisioning/messages"

	nats "github.com/nats-io/nats.go"
)

type NATSBus struct {
	conn              *nats.Conn
	js                nats.JetStreamContext
	commandStreamName string
}

type NATSBusOption func(*NATSBus)

func WithCommandStreamName(name string) NATSBusOption {
	return func(b *NATSBus) {
		if name != "" {
			b.commandStreamName = name
		}
	}
}

func NewNATSBus(url, credentials string, opts ...NATSBusOption) (*NATSBus, error) {
	natsOptions := []nats.Option{}
	if credentials != "" {
		natsOptions = append(natsOptions, nats.UserCredentials(credentials))
	}
	conn, err := nats.Connect(url, natsOptions...)
	if err != nil {
		return nil, err
	}
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, err
	}
	bus := &NATSBus{conn: conn, js: js, commandStreamName: messages.DefaultCommandStreamName}
	for _, opt := range opts {
		opt(bus)
	}
	if err := EnsureCommandStream(js, bus.commandStreamName); err != nil {
		conn.Close()
		return nil, err
	}
	return bus, nil
}

func (b *NATSBus) PublishCommand(ctx context.Context, subject string, envelope messages.Envelope) error {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	_, err = b.js.Publish(subject, payload, nats.MsgId(envelope.MessageID))
	return err
}

func (b *NATSBus) Request(ctx context.Context, subject string, envelope messages.Envelope, timeout time.Duration) (messages.Envelope, error) {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return messages.Envelope{}, err
	}
	msg, err := b.conn.RequestWithContext(ctx, subject, payload)
	if err != nil {
		return messages.Envelope{}, err
	}
	var reply messages.Envelope
	return reply, json.Unmarshal(msg.Data, &reply)
}

func (b *NATSBus) Close() {
	b.conn.Drain()
	b.conn.Close()
}

func EnsureCommandStream(js nats.JetStreamContext, streamName string) error {
	if streamName == "" {
		streamName = messages.DefaultCommandStreamName
	}
	config := &nats.StreamConfig{
		Name:       streamName,
		Subjects:   messages.CommandStreamSubjects(),
		Retention:  nats.WorkQueuePolicy,
		Storage:    nats.FileStorage,
		MaxAge:     24 * time.Hour,
		Duplicates: 2 * time.Hour,
	}
	info, err := js.StreamInfo(streamName)
	if err == nil {
		info.Config.Subjects = config.Subjects
		info.Config.Retention = config.Retention
		info.Config.Storage = config.Storage
		info.Config.MaxAge = config.MaxAge
		info.Config.Duplicates = config.Duplicates
		_, err = js.UpdateStream(&info.Config)
		return err
	}
	if err == nats.ErrStreamNotFound {
		_, err = js.AddStream(config)
	}
	return err
}
