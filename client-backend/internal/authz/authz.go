package authz

import (
	"context"

	"github.com/google/uuid"
)

type ResourceChecker interface {
	ResourceBelongsToOrganization(ctx context.Context, resourceType string, resourceID, organizationID uuid.UUID) (bool, error)
}

func RequireResourceInOrganization(ctx context.Context, checker ResourceChecker, resourceType string, resourceID, organizationID uuid.UUID) error {
	ok, err := checker.ResourceBelongsToOrganization(ctx, resourceType, resourceID, organizationID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrResourceOutsideOrganization
	}
	return nil
}

var ErrResourceOutsideOrganization = errString("resource does not belong to organization")

type errString string

func (e errString) Error() string { return string(e) }
