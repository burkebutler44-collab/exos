package central

import (
	"context"
	"time"

	"relay/client-backend/internal/provisioning/messages"
)

type Repository interface {
	GetRack(ctx context.Context, rackID string) (Rack, error)
	ListRacks(ctx context.Context) ([]Rack, error)
	UpdateRackHeartbeat(ctx context.Context, heartbeat messages.RackHeartbeat) (Rack, RackAgent, error)
	MarkRacksOfflineBefore(ctx context.Context, cutoff time.Time) error
	SetRackStatus(ctx context.Context, rackID string, status RackStatus) (Rack, error)

	GetServerForOrganization(ctx context.Context, organizationID, serverID string) (Server, error)
	ReserveServerForProvisioning(ctx context.Context, organizationID, serverID string) (Server, error)
	UpdateServerStatus(ctx context.Context, serverID string, status ServerStatus) error

	CreateProvisioningJob(ctx context.Context, input CreateProvisioningJobInput) (ProvisioningJob, error)
	GetProvisioningJob(ctx context.Context, organizationID, jobID string) (ProvisioningJob, error)
	GetActiveProvisioningJobForServer(ctx context.Context, serverID string) (*ProvisioningJob, error)
	UpdateProvisioningJobCommand(ctx context.Context, jobID, messageID string, status JobStatus) (ProvisioningJob, error)
	ApplyProvisioningEvent(ctx context.Context, envelope messages.Envelope, event messages.ProvisioningEvent) (ProvisioningJob, bool, error)
	ListProvisioningJobEvents(ctx context.Context, organizationID, jobID string) ([]ProvisioningJobEvent, error)

	RecordRackMessage(ctx context.Context, message RackMessage) (bool, error)
	MarkRackMessageProcessed(ctx context.Context, messageID string, status MessageStatus) error
}

type CreateProvisioningJobInput struct {
	OrganizationID    string
	ProjectID         *string
	ServerID          string
	RackID            string
	RequestedByUserID string
	ImageID           string
	Hostname          string
	CorrelationID     string
	ExpiresAt         *time.Time
}
