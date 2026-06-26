package central

import (
	"encoding/json"
	"time"
)

type RackStatus string
type AgentStatus string
type ServerStatus string
type JobStatus string
type MessageDirection string
type MessageStatus string

const (
	RackOnline      RackStatus = "online"
	RackDegraded    RackStatus = "degraded"
	RackOffline     RackStatus = "offline"
	RackMaintenance RackStatus = "maintenance"

	AgentOnline   AgentStatus = "online"
	AgentDegraded AgentStatus = "degraded"
	AgentOffline  AgentStatus = "offline"

	ServerAvailable             ServerStatus = "available"
	ServerReserved              ServerStatus = "reserved"
	ServerProvisioningRequested ServerStatus = "provisioning_requested"
	ServerProvisioningStarted   ServerStatus = "provisioning_started"
	ServerPXEBooting            ServerStatus = "pxe_booting"
	ServerInstalling            ServerStatus = "installing"
	ServerActive                ServerStatus = "active"
	ServerFailed                ServerStatus = "failed"
	ServerSuspended             ServerStatus = "suspended"
	ServerCanceled              ServerStatus = "canceled"
	ServerTerminated            ServerStatus = "terminated"

	JobPending          JobStatus = "pending"
	JobCommandPublished JobStatus = "command_published"
	JobAcceptedByRack   JobStatus = "accepted_by_rack"
	JobRunning          JobStatus = "running"
	JobCompleted        JobStatus = "completed"
	JobFailed           JobStatus = "failed"
	JobExpired          JobStatus = "expired"
	JobCanceled         JobStatus = "canceled"

	DirectionCentralToRack MessageDirection = "central_to_rack"
	DirectionRackToCentral MessageDirection = "rack_to_central"

	MessageReceived         MessageStatus = "received"
	MessageProcessed        MessageStatus = "processed"
	MessageIgnoredDuplicate MessageStatus = "ignored_duplicate"
	MessageFailed           MessageStatus = "failed"
	MessageExpired          MessageStatus = "expired"
)

type Rack struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Location        string          `json:"location"`
	Status          RackStatus      `json:"status"`
	LastHeartbeatAt *time.Time      `json:"last_heartbeat_at,omitempty"`
	LastSeenAt      *time.Time      `json:"last_seen_at,omitempty"`
	Metadata        json.RawMessage `json:"metadata"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type RackAgent struct {
	ID              string          `json:"id"`
	RackID          string          `json:"rack_id"`
	AgentID         string          `json:"agent_id"`
	Version         string          `json:"version"`
	Status          AgentStatus     `json:"status"`
	LastHeartbeatAt *time.Time      `json:"last_heartbeat_at,omitempty"`
	Metadata        json.RawMessage `json:"metadata"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type Server struct {
	ID             string          `json:"id"`
	OrganizationID *string         `json:"organization_id,omitempty"`
	ProjectID      *string         `json:"project_id,omitempty"`
	RackID         string          `json:"rack_id"`
	Status         ServerStatus    `json:"status"`
	BMCAddress     string          `json:"bmc_address"`
	MACAddress     string          `json:"mac_address"`
	Provisionable  bool            `json:"provisionable"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type ProvisioningJob struct {
	ID                string     `json:"id"`
	OrganizationID    string     `json:"organization_id"`
	ProjectID         *string    `json:"project_id,omitempty"`
	ServerID          string     `json:"server_id"`
	RackID            string     `json:"rack_id"`
	RequestedByUserID string     `json:"requested_by_user_id"`
	ImageID           string     `json:"image_id"`
	Hostname          string     `json:"hostname"`
	Status            JobStatus  `json:"status"`
	FailureReason     *string    `json:"failure_reason,omitempty"`
	CorrelationID     string     `json:"correlation_id"`
	CommandMessageID  *string    `json:"command_message_id,omitempty"`
	RequestedAt       time.Time  `json:"requested_at"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type ProvisioningJobEvent struct {
	ID                string          `json:"id"`
	ProvisioningJobID string          `json:"provisioning_job_id"`
	EventType         string          `json:"event_type"`
	Message           string          `json:"message"`
	Metadata          json.RawMessage `json:"metadata"`
	CreatedAt         time.Time       `json:"created_at"`
}

type RackMessage struct {
	ID          string           `json:"id"`
	MessageID   string           `json:"message_id"`
	Direction   MessageDirection `json:"direction"`
	RackID      string           `json:"rack_id"`
	ServerID    *string          `json:"server_id,omitempty"`
	JobID       *string          `json:"job_id,omitempty"`
	MessageType string           `json:"message_type"`
	Status      MessageStatus    `json:"status"`
	Payload     json.RawMessage  `json:"payload"`
	ProcessedAt *time.Time       `json:"processed_at,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
}
