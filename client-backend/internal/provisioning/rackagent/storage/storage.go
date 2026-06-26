package storage

import (
	"context"
	"time"
)

type LocalJob struct {
	ID               string
	CentralJobID     string
	CommandMessageID string
	RackID           string
	ServerID         string
	Status           string
	LastStep         string
	FailureReason    *string
	StartedAt        *time.Time
	CompletedAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ProcessedMessage struct {
	ID           string
	MessageID    string
	MessageType  string
	ProcessedAt  time.Time
	ResultStatus string
	PayloadHash  string
	CreatedAt    time.Time
}

type LocalJobRepository interface {
	CreateOrGetJob(ctx context.Context, job LocalJob) (LocalJob, bool, error)
	UpdateJobStep(ctx context.Context, centralJobID, status, step string, failureReason *string) error
	GetJobByCentralID(ctx context.Context, centralJobID string) (LocalJob, error)
	ActiveJobsCount(ctx context.Context) (int, error)
}

type ProcessedMessageRepository interface {
	AlreadyProcessed(ctx context.Context, messageID string) (bool, error)
	MarkProcessed(ctx context.Context, message ProcessedMessage) error
}
