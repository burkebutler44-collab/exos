package rackagent

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"relay/client-backend/internal/provisioning/messages"

	"github.com/google/uuid"
	nats "github.com/nats-io/nats.go"
)

type NATSPublisher struct {
	conn     *nats.Conn
	subjects messages.SubjectBuilder
	location string
}

func NewNATSPublisher(conn *nats.Conn, location string) *NATSPublisher {
	return &NATSPublisher{conn: conn, subjects: messages.SubjectBuilder{}, location: location}
}

func (p *NATSPublisher) PublishEvent(ctx context.Context, kind messages.EventKind, envelope messages.Envelope) error {
	if p.location != "" && kind == messages.EventProvision {
		return p.publish(ctx, p.subjects.DataCenterProvisionStatus(p.location), envelope)
	}
	return p.publish(ctx, p.subjects.Event(envelope.RackID, kind), envelope)
}

func (p *NATSPublisher) PublishHeartbeat(ctx context.Context, envelope messages.Envelope) error {
	if p.location != "" {
		return p.publish(ctx, p.subjects.DataCenterHeartbeat(p.location), envelope)
	}
	return p.publish(ctx, p.subjects.Heartbeat(envelope.RackID), envelope)
}

func (p *NATSPublisher) publish(ctx context.Context, subject string, envelope messages.Envelope) error {
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

func ConnectNATS(url, credentials string) (*nats.Conn, error) {
	opts := []nats.Option{}
	if credentials != "" {
		opts = append(opts, nats.UserCredentials(credentials))
	}
	return nats.Connect(url, opts...)
}

func RunNATS(ctx context.Context, agent *Agent, conn *nats.Conn) error {
	subjects := messages.SubjectBuilder{}
	js, err := conn.JetStream()
	if err != nil {
		return err
	}
	streamName := agent.CommandStream
	if streamName == "" {
		streamName = messages.DefaultCommandStreamName
	}
	if err := ensureCommandStream(js, streamName); err != nil {
		return err
	}
	ackWait := agent.CommandAckWait
	if ackWait == 0 {
		ackWait = 10 * time.Minute
	}

	provisionSub, err := subscribeDurableCommand(js, streamName, subjects.DataCenterProvisionRequest(agent.Location), commandDurable(agent, "provision"), ackWait)
	if err != nil {
		return err
	}
	defer provisionSub.Unsubscribe()

	powerSub, err := subscribeDurableCommand(js, streamName, subjects.DataCenterServerPower(agent.Location), commandDurable(agent, "power"), ackWait)
	if err != nil {
		_ = provisionSub.Unsubscribe()
		return err
	}
	defer powerSub.Unsubscribe()

	errCh := make(chan error, 2)
	go runCommandConsumer(ctx, agent, provisionSub, subjects.DataCenterProvisionRequest(agent.Location), messages.ProvisionServerCommandType, errCh)
	go runCommandConsumer(ctx, agent, powerSub, subjects.DataCenterServerPower(agent.Location), messages.PowerCommandType, errCh)

	requestSubs, err := subscribeRequests(ctx, agent, conn)
	if err != nil {
		return err
	}
	defer func() {
		for _, sub := range requestSubs {
			_ = sub.Unsubscribe()
		}
	}()

	ticker := time.NewTicker(agent.HeartbeatEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if err != nil {
				return err
			}
		case <-ticker.C:
			if err := agent.PublishHeartbeat(ctx); err != nil {
				log.Printf("rack_agent heartbeat publish failed: %v", err)
			}
		}
	}
}

func ensureCommandStream(js nats.JetStreamContext, streamName string) error {
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

func subscribeDurableCommand(js nats.JetStreamContext, streamName, subject, durable string, ackWait time.Duration) (*nats.Subscription, error) {
	return js.PullSubscribe(
		subject,
		durable,
		nats.BindStream(streamName),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.AckWait(ackWait),
		nats.MaxDeliver(20),
	)
}

func runCommandConsumer(ctx context.Context, agent *Agent, sub *nats.Subscription, subject string, fallbackType string, errCh chan<- error) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		msgs, err := sub.Fetch(1, nats.MaxWait(time.Second))
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			errCh <- err
			return
		}
		for _, msg := range msgs {
			handleJetStreamCommand(ctx, agent, msg, subject, fallbackType)
		}
	}
}

func handleJetStreamCommand(ctx context.Context, agent *Agent, msg *nats.Msg, subject string, fallbackType string) {
	env, err := commandEnvelopeFromNATS(msg.Data, agent, fallbackType)
	if err != nil {
		log.Printf("rack_agent command decode failed subject=%s: %v", subject, err)
		_ = msg.Ack()
		return
	}
	err = agent.HandleCommand(ctx, env)
	if err == nil || errors.Is(err, ErrDuplicateCommand) || errors.Is(err, ErrExpiredCommand) || errors.Is(err, ErrWrongRack) {
		_ = msg.Ack()
		return
	}
	seen, seenErr := agent.Processed.AlreadyProcessed(ctx, env.MessageID)
	if seenErr == nil && seen {
		_ = msg.Ack()
		return
	}
	log.Printf("rack_agent command failed subject=%s message_id=%s type=%s rack=%s: %v", subject, env.MessageID, env.MessageType, env.RackID, err)
	_ = msg.Nak()
}

func commandDurable(agent *Agent, suffix string) string {
	prefix := agent.CommandDurable
	if prefix == "" {
		prefix = "rack-agent-" + agent.Location
	}
	return prefix + "-" + suffix
}

func commandEnvelopeFromNATS(data []byte, agent *Agent, fallbackType string) (messages.Envelope, error) {
	var env messages.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return messages.Envelope{}, err
	}
	now := time.Now().UTC()
	if env.MessageID == "" && len(env.Payload) == 0 {
		return messages.Envelope{
			MessageID:     uuid.NewString(),
			MessageType:   fallbackType,
			RackID:        agent.Location,
			CreatedAt:     now,
			SchemaVersion: messages.SchemaVersion,
			Payload:       normalizeBareCommandPayload(data, fallbackType),
			Metadata:      map[string]string{"source": "bare_nats_payload"},
		}, nil
	}
	if env.MessageType == "" {
		env.MessageType = fallbackType
	}
	if env.RackID == "" {
		env.RackID = agent.Location
	}
	if env.CreatedAt.IsZero() {
		env.CreatedAt = now
	}
	if env.SchemaVersion == "" {
		env.SchemaVersion = messages.SchemaVersion
	}
	return env, nil
}

func normalizeBareCommandPayload(data []byte, fallbackType string) json.RawMessage {
	if fallbackType != messages.ProvisionServerCommandType {
		return json.RawMessage(data)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return json.RawMessage(data)
	}
	if _, ok := fields["image_id"]; !ok {
		if image, ok := fields["image"]; ok {
			fields["image_id"] = image
		}
	}
	if _, ok := fields["hostname"]; !ok {
		if serverID, ok := fields["server_id"]; ok {
			fields["hostname"] = serverID
		}
	}
	networkConfig := map[string]any{}
	if existing, ok := fields["network_config"].(map[string]any); ok {
		for key, value := range existing {
			networkConfig[key] = value
		}
	}
	for _, key := range []string{"mac", "ip", "ip_address", "address", "gateway", "netmask", "mask"} {
		if value, ok := fields[key]; ok {
			networkConfig[key] = value
		}
	}
	if len(networkConfig) > 0 {
		fields["network_config"] = networkConfig
	}
	normalized, err := json.Marshal(fields)
	if err != nil {
		return json.RawMessage(data)
	}
	return normalized
}

func subscribeRequests(ctx context.Context, agent *Agent, conn *nats.Conn) ([]*nats.Subscription, error) {
	subjects := messages.SubjectBuilder{}
	kinds := []messages.RequestKind{
		messages.RequestHealth,
		messages.RequestPowerState,
		messages.RequestBMCCheck,
		messages.RequestProvisioningStatus,
		messages.RequestTinkerbellHealth,
	}
	subs := make([]*nats.Subscription, 0, len(kinds))
	for _, kind := range kinds {
		requestKind := kind
		sub, err := conn.Subscribe(subjects.DataCenterRequest(agent.Location, requestKind), func(msg *nats.Msg) {
			var env messages.Envelope
			if err := json.Unmarshal(msg.Data, &env); err != nil {
				log.Printf("rack_agent request decode failed: %v", err)
				return
			}
			reply, err := agent.RespondToRequest(ctx, requestKind, env)
			if err != nil {
				log.Printf("rack_agent request failed kind=%s: %v", requestKind, err)
				return
			}
			payload, err := json.Marshal(reply)
			if err != nil {
				log.Printf("rack_agent request reply encode failed kind=%s: %v", requestKind, err)
				return
			}
			if err := msg.Respond(payload); err != nil {
				log.Printf("rack_agent request reply publish failed kind=%s: %v", requestKind, err)
			}
		})
		if err != nil {
			for _, existing := range subs {
				_ = existing.Unsubscribe()
			}
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, nil
}
