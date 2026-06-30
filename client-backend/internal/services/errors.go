package services

import "errors"

var (
	ErrForbidden             = errors.New("forbidden")
	ErrNotFound              = errors.New("not found")
	ErrConflict              = errors.New("conflict")
	ErrInvalidInput          = errors.New("invalid input")
	ErrLastOwner             = errors.New("last owner cannot be removed or downgraded")
	ErrBillableResources     = errors.New("organization has active billable resources")
	ErrInvitationExpired     = errors.New("invitation expired")
	ErrInvitationNotPending  = errors.New("invitation is not pending")
	ErrFinalizedInvoice      = errors.New("finalized invoice line items are immutable")
	ErrInsufficientBalance   = errors.New("insufficient prepaid balance")
	ErrDuplicateWebhook      = errors.New("stripe webhook already processed")
	ErrStripeNotConfigured   = errors.New("stripe is not configured")
	ErrStripeRequestFailed   = errors.New("stripe request failed")
	ErrPaymentMethodRequired = errors.New("payment method required")
)
