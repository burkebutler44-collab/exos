package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	Auth0Sub  string    `json:"auth0_sub"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Organization struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Slug            string    `json:"slug"`
	CreatedByUserID uuid.UUID `json:"created_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Role struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	Name           string     `json:"name"`
	IsSystemRole   bool       `json:"is_system_role"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type MembershipStatus string

const (
	MembershipActive    MembershipStatus = "active"
	MembershipInvited   MembershipStatus = "invited"
	MembershipSuspended MembershipStatus = "suspended"
	MembershipRemoved   MembershipStatus = "removed"
)

type Member struct {
	ID             uuid.UUID        `json:"id"`
	OrganizationID uuid.UUID        `json:"organization_id"`
	UserID         uuid.UUID        `json:"user_id"`
	RoleID         uuid.UUID        `json:"role_id"`
	RoleName       string           `json:"role_name"`
	Status         MembershipStatus `json:"status"`
	JoinedAt       *time.Time       `json:"joined_at,omitempty"`
	Email          string           `json:"email"`
	Name           string           `json:"name"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
	InvitationExpired  InvitationStatus = "expired"
	InvitationRevoked  InvitationStatus = "revoked"
)

type Invitation struct {
	ID              uuid.UUID        `json:"id"`
	OrganizationID  uuid.UUID        `json:"organization_id"`
	Email           string           `json:"email"`
	RoleID          uuid.UUID        `json:"role_id"`
	RoleName        string           `json:"role_name"`
	InvitedByUserID uuid.UUID        `json:"invited_by_user_id"`
	Token           string           `json:"token,omitempty"`
	Status          InvitationStatus `json:"status"`
	ExpiresAt       time.Time        `json:"expires_at"`
	AcceptedAt      *time.Time       `json:"accepted_at,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type Project struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BillingProfile struct {
	ID               uuid.UUID `json:"id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	StripeCustomerID *string   `json:"stripe_customer_id,omitempty"`
	BillingEmail     string    `json:"billing_email"`
	CompanyName      string    `json:"company_name"`
	TaxID            *string   `json:"tax_id,omitempty"`
	Line1            *string   `json:"line1,omitempty"`
	Line2            *string   `json:"line2,omitempty"`
	City             *string   `json:"city,omitempty"`
	State            *string   `json:"state,omitempty"`
	PostalCode       *string   `json:"postal_code,omitempty"`
	Country          *string   `json:"country,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PaymentMethod struct {
	ID                    uuid.UUID `json:"id"`
	OrganizationID        uuid.UUID `json:"organization_id"`
	StripePaymentMethodID string    `json:"stripe_payment_method_id"`
	Brand                 string    `json:"brand"`
	Last4                 string    `json:"last4"`
	ExpMonth              int32     `json:"exp_month"`
	ExpYear               int32     `json:"exp_year"`
	IsDefault             bool      `json:"is_default"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type Invoice struct {
	ID              uuid.UUID `json:"id"`
	OrganizationID  uuid.UUID `json:"organization_id"`
	StripeInvoiceID string    `json:"stripe_invoice_id"`
	Status          string    `json:"status"`
	AmountDue       int64     `json:"amount_due"`
	AmountPaid      int64     `json:"amount_paid"`
	PeriodStart     time.Time `json:"period_start"`
	PeriodEnd       time.Time `json:"period_end"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type AuditLogEntry struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	ActorUserID    *uuid.UUID      `json:"actor_user_id,omitempty"`
	Action         string          `json:"action"`
	EntityType     string          `json:"entity_type"`
	EntityID       *uuid.UUID      `json:"entity_id,omitempty"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"created_at"`
}

type ResourceRef struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	ProjectID      *uuid.UUID
}
