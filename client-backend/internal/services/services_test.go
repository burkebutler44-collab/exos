package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/store"

	"github.com/google/uuid"
)

type fakeRepo struct {
	store.Repository

	ownerRoleID uuid.UUID
	adminRoleID uuid.UUID
	members     []domain.Member
	permissions bool
	projects    map[string]bool

	creatorBecameOwner bool
	billingCreated     bool
	acceptedMembership bool
	resourceOK         bool
}

func TestOrganizationCreatorBecomesOwner(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo)
	user := domain.User{ID: uuid.New(), Email: "founder@example.com"}

	org, err := svc.CreateOrganization(context.Background(), user, "Exos Labs")
	if err != nil {
		t.Fatalf("CreateOrganization returned error: %v", err)
	}
	if org.CreatedByUserID != user.ID {
		t.Fatalf("created_by_user_id = %s, want %s", org.CreatedByUserID, user.ID)
	}
	if !repo.creatorBecameOwner {
		t.Fatal("creator was not assigned Owner membership")
	}
	if !repo.billingCreated {
		t.Fatal("billing profile placeholder was not created")
	}
}

func TestUserWithoutPermissionIsDenied(t *testing.T) {
	repo := &fakeRepo{permissions: false}
	svc := New(repo)

	err := svc.RequirePermission(context.Background(), uuid.New(), uuid.New(), domain.PermissionBillingManage)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("RequirePermission error = %v, want ErrForbidden", err)
	}
}

func TestLastOwnerCannotBeRemoved(t *testing.T) {
	ownerRoleID := uuid.New()
	userID := uuid.New()
	repo := &fakeRepo{
		ownerRoleID: ownerRoleID,
		members: []domain.Member{{
			ID: uuid.New(), UserID: userID, RoleID: ownerRoleID, Status: domain.MembershipActive,
		}},
	}
	svc := New(repo)

	err := svc.RemoveMember(context.Background(), domain.User{ID: uuid.New()}, uuid.New(), userID)
	if !errors.Is(err, ErrLastOwner) {
		t.Fatalf("RemoveMember error = %v, want ErrLastOwner", err)
	}
}

func TestLastOwnerCannotBeDowngraded(t *testing.T) {
	ownerRoleID := uuid.New()
	adminRoleID := uuid.New()
	userID := uuid.New()
	repo := &fakeRepo{
		ownerRoleID: ownerRoleID,
		adminRoleID: adminRoleID,
		members: []domain.Member{{
			ID: uuid.New(), UserID: userID, RoleID: ownerRoleID, Status: domain.MembershipActive,
		}},
	}
	svc := New(repo)

	_, err := svc.UpdateMemberRole(context.Background(), domain.User{ID: uuid.New()}, uuid.New(), userID, adminRoleID)
	if !errors.Is(err, ErrLastOwner) {
		t.Fatalf("UpdateMemberRole error = %v, want ErrLastOwner", err)
	}
}

func TestProjectSlugIsUniquePerOrganization(t *testing.T) {
	repo := &fakeRepo{projects: map[string]bool{}}
	svc := New(repo)
	orgID := uuid.New()
	actor := domain.User{ID: uuid.New()}

	if _, err := svc.CreateProject(context.Background(), actor, orgID, "Production"); err != nil {
		t.Fatalf("first CreateProject returned error: %v", err)
	}
	_, err := svc.CreateProject(context.Background(), actor, orgID, "Production")
	if !IsConflict(err) {
		t.Fatalf("second CreateProject error = %v, want conflict", err)
	}
}

func TestResourceAccessRequiresMatchingOrganization(t *testing.T) {
	repo := &fakeRepo{resourceOK: false}
	svc := New(repo)

	ok, err := svc.ResourceBelongsToOrganization(context.Background(), "servers", uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("ResourceBelongsToOrganization returned error: %v", err)
	}
	if ok {
		t.Fatal("resource was allowed without matching organization")
	}
}

func TestInvitationAcceptCreatesMembership(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo)
	user := domain.User{ID: uuid.New(), Email: "teammate@example.com"}

	_, err := svc.AcceptInvitation(context.Background(), user, "token")
	if err != nil {
		t.Fatalf("AcceptInvitation returned error: %v", err)
	}
	if !repo.acceptedMembership {
		t.Fatal("accepting invitation did not create or activate membership")
	}
}

func (f *fakeRepo) CreateOrganization(ctx context.Context, params store.CreateOrganizationParams) (domain.Organization, error) {
	f.creatorBecameOwner = params.CreatedByUserID != uuid.Nil
	f.billingCreated = params.BillingEmail != ""
	return domain.Organization{ID: uuid.New(), Name: params.Name, Slug: params.Slug, CreatedByUserID: params.CreatedByUserID}, nil
}

func (f *fakeRepo) HasPermission(ctx context.Context, userID, organizationID uuid.UUID, permission string) (bool, error) {
	return f.permissions, nil
}

func (f *fakeRepo) ListMembers(ctx context.Context, organizationID uuid.UUID) ([]domain.Member, error) {
	return f.members, nil
}

func (f *fakeRepo) IsOwnerRole(ctx context.Context, roleID uuid.UUID) (bool, error) {
	return roleID == f.ownerRoleID, nil
}

func (f *fakeRepo) CountActiveOwners(ctx context.Context, organizationID uuid.UUID) (int64, error) {
	var owners int64
	for _, member := range f.members {
		if member.RoleID == f.ownerRoleID && member.Status == domain.MembershipActive {
			owners++
		}
	}
	return owners, nil
}

func (f *fakeRepo) CreateProject(ctx context.Context, organizationID uuid.UUID, name, slug string) (domain.Project, error) {
	key := organizationID.String() + "/" + slug
	if f.projects[key] {
		return domain.Project{}, store.ErrConflict
	}
	f.projects[key] = true
	return domain.Project{ID: uuid.New(), OrganizationID: organizationID, Name: name, Slug: slug}, nil
}

func (f *fakeRepo) ResourceBelongsToOrganization(ctx context.Context, resourceType string, resourceID, organizationID uuid.UUID) (bool, error) {
	return f.resourceOK, nil
}

func (f *fakeRepo) GetInvitationByToken(ctx context.Context, token string) (domain.Invitation, error) {
	return domain.Invitation{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Email:          "teammate@example.com",
		RoleID:         uuid.New(),
		Token:          token,
		Status:         domain.InvitationPending,
		ExpiresAt:      time.Now().Add(time.Hour),
	}, nil
}

func (f *fakeRepo) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) (domain.Invitation, error) {
	f.acceptedMembership = userID != uuid.Nil
	return domain.Invitation{Token: token, Status: domain.InvitationAccepted}, nil
}

func (f *fakeRepo) AddAuditLog(ctx context.Context, organizationID uuid.UUID, actorUserID *uuid.UUID, action, entityType string, entityID *uuid.UUID, metadata []byte) error {
	return nil
}
