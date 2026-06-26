package messages

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

const SchemaVersion = "v1"

var (
	ErrMissingMessageID = errors.New("message_id is required")
	ErrMissingType      = errors.New("message_type is required")
	ErrMissingRackID    = errors.New("rack_id is required")
	ErrExpiredMessage   = errors.New("message is expired")
)

type Envelope struct {
	MessageID     string            `json:"message_id"`
	CorrelationID *string           `json:"correlation_id,omitempty"`
	CausationID   *string           `json:"causation_id,omitempty"`
	MessageType   string            `json:"message_type"`
	RackID        string            `json:"rack_id"`
	ServerID      *string           `json:"server_id,omitempty"`
	JobID         *string           `json:"job_id,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
	SchemaVersion string            `json:"schema_version"`
	Payload       json.RawMessage   `json:"payload"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

func NewEnvelope(messageType, rackID string, payload any) (Envelope, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		MessageID:     uuid.NewString(),
		MessageType:   messageType,
		RackID:        rackID,
		CreatedAt:     time.Now().UTC(),
		SchemaVersion: SchemaVersion,
		Payload:       encoded,
		Metadata:      map[string]string{},
	}, nil
}

func (e Envelope) Validate(now time.Time) error {
	if e.MessageID == "" {
		return ErrMissingMessageID
	}
	if e.MessageType == "" {
		return ErrMissingType
	}
	if e.RackID == "" {
		return ErrMissingRackID
	}
	if e.ExpiresAt != nil && now.After(*e.ExpiresAt) {
		return ErrExpiredMessage
	}
	return nil
}

func (e Envelope) DecodePayload(target any) error {
	return json.Unmarshal(e.Payload, target)
}

func (e Envelope) WithCausation(messageID string) Envelope {
	e.CausationID = &messageID
	return e
}

func (e Envelope) WithCorrelation(correlationID string) Envelope {
	e.CorrelationID = &correlationID
	return e
}
