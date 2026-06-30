package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/store"

	"github.com/google/uuid"
)

type Services struct {
	repo Repository
}

type Repository interface {
	store.Repository
}

func New(repo Repository) *Services {
	return &Services{repo: repo}
}

func (s *Services) UpsertUser(ctx context.Context, identity store.AuthIdentity) (domain.User, error) {
	if strings.TrimSpace(identity.Auth0Sub) == "" {
		return domain.User{}, ErrInvalidInput
	}
	if identity.Email == "" {
		identity.Email = identity.Auth0Sub + "@relay.local"
	}
	return s.repo.UpsertUser(ctx, identity)
}

func (s *Services) CreateOrganization(ctx context.Context, user domain.User, name string) (domain.Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Organization{}, ErrInvalidInput
	}
	return s.repo.CreateOrganization(ctx, store.CreateOrganizationParams{
		Name:            name,
		Slug:            slugify(name),
		CreatedByUserID: user.ID,
		BillingEmail:    user.Email,
	})
}

func (s *Services) ListOrganizations(ctx context.Context, user domain.User) ([]domain.Organization, error) {
	return s.repo.ListOrganizationsForUser(ctx, user.ID)
}

func (s *Services) ListServerCatalog(ctx context.Context) (store.ServerCatalog, error) {
	return s.repo.ListServerCatalog(ctx)
}

func (s *Services) AllocateServer(ctx context.Context, actor domain.User, params store.AllocateServerParams) (store.AllocateServerResult, error) {
	if actor.ID == uuid.Nil || params.OrganizationID == uuid.Nil || params.ServerFamilyID == uuid.Nil || strings.TrimSpace(params.ConfigurationID) == "" {
		return store.AllocateServerResult{}, ErrInvalidInput
	}
	client, err := newStripeClientFromEnv()
	if err != nil {
		return store.AllocateServerResult{}, err
	}
	account, stripeCustomerID, err := s.ensureStripeCustomer(ctx, params.OrganizationID, client)
	if err != nil {
		return store.AllocateServerResult{}, err
	}
	methods, err := s.repo.ListPaymentMethods(ctx, params.OrganizationID)
	if err != nil {
		return store.AllocateServerResult{}, err
	}
	var paymentMethod *domain.PaymentMethod
	for i := range methods {
		if methods[i].IsDefault {
			paymentMethod = &methods[i]
			break
		}
	}
	if paymentMethod == nil && len(methods) > 0 {
		paymentMethod = &methods[0]
	}
	if paymentMethod == nil {
		return store.AllocateServerResult{}, ErrPaymentMethodRequired
	}

	params.CreatedByUserID = actor.ID
	result, err := s.repo.AllocateServer(ctx, params)
	if err != nil {
		return store.AllocateServerResult{}, err
	}
	description := checkoutDescription(result.Order)
	intent, err := client.createAndConfirmPaymentIntent(ctx, stripePaymentIntentParams{
		CustomerID:     stripeCustomerID,
		PaymentMethod:  paymentMethod.StripePaymentMethodID,
		OrganizationID: params.OrganizationID.String(),
		OrderID:        result.Order.ID.String(),
		Description:    description,
		AmountCents:    result.Order.TotalCents,
		Currency:       account.Currency,
	})
	if err != nil {
		return store.AllocateServerResult{}, err
	}
	if intent.Status != "succeeded" {
		return store.AllocateServerResult{}, fmt.Errorf("%w: payment requires customer action", ErrStripeRequestFailed)
	}
	result.Order, err = s.repo.SetOrderStripePaymentIntent(ctx, params.OrganizationID, result.Order.ID, intent.ID)
	if err != nil {
		return store.AllocateServerResult{}, err
	}
	result.Order, err = s.repo.MarkOrderPaidAndActivate(ctx, params.OrganizationID, result.Order.ID, &intent.ID)
	if err != nil {
		return store.AllocateServerResult{}, err
	}
	_ = s.audit(ctx, params.OrganizationID, &actor.ID, "server.ordered", "server", &result.Server.ID, map[string]string{
		"order_id":         result.Order.ID.String(),
		"server_family_id": params.ServerFamilyID.String(),
		"configuration_id": params.ConfigurationID,
	})
	return result, nil
}

func (s *Services) ensureStripeCustomer(ctx context.Context, organizationID uuid.UUID, client *stripeClient) (domain.BillingAccount, string, error) {
	account, err := s.repo.GetBillingAccount(ctx, organizationID)
	if err != nil {
		return domain.BillingAccount{}, "", err
	}
	if account.StripeCustomerID != nil && strings.TrimSpace(*account.StripeCustomerID) != "" {
		return account, strings.TrimSpace(*account.StripeCustomerID), nil
	}
	organization, err := s.repo.GetOrganizationByID(ctx, organizationID)
	if err != nil {
		return domain.BillingAccount{}, "", err
	}
	customer, err := client.createCustomer(ctx, stripeCustomerParams{
		Email:          account.BillingEmail,
		Name:           organization.Name,
		OrganizationID: organizationID.String(),
	})
	if err != nil {
		return domain.BillingAccount{}, "", err
	}
	account, err = s.repo.SetBillingAccountStripeCustomerID(ctx, organizationID, customer.ID)
	if err != nil {
		return domain.BillingAccount{}, "", err
	}
	return account, customer.ID, nil
}

func checkoutDescription(order domain.Order) string {
	var pending store.PendingServiceMetadata
	if err := json.Unmarshal(order.Metadata, &pending); err != nil || pending.Description == "" {
		return "Initial prorated server charge"
	}
	if pending.FirstPeriodStart != nil && pending.FirstPeriodEnd != nil && pending.FirstPeriodHours > 0 {
		return fmt.Sprintf("%s - prorated %d/%d hours through %s", pending.Description, pending.FirstPeriodHours, pending.MonthlyHours, pending.FirstPeriodEnd.Format("2006-01-02"))
	}
	return pending.Description
}

func frontendURL(organizationID uuid.UUID, segment string, values map[string]string) string {
	base := strings.TrimRight(os.Getenv("FRONTEND_BASE_URL"), "/")
	if base == "" {
		base = "http://localhost:5173"
	}
	path := fmt.Sprintf("%s/%s/%s", base, url.PathEscape(organizationID.String()), strings.Trim(segment, "/"))
	query := url.Values{}
	for key, value := range values {
		query.Set(key, value)
	}
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

func (s *Services) GetOrganization(ctx context.Context, user domain.User, organizationID uuid.UUID) (domain.Organization, error) {
	return s.repo.GetOrganizationForUser(ctx, organizationID, user.ID)
}

func (s *Services) GetOrganizationByID(ctx context.Context, organizationID uuid.UUID) (domain.Organization, error) {
	return s.repo.GetOrganizationByID(ctx, organizationID)
}

func (s *Services) UpdateOrganization(ctx context.Context, actor domain.User, organizationID uuid.UUID, name string) (domain.Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Organization{}, ErrInvalidInput
	}
	org, err := s.repo.UpdateOrganization(ctx, organizationID, name, slugify(name))
	if err != nil {
		return domain.Organization{}, err
	}
	_ = s.audit(ctx, organizationID, &actor.ID, "organization.updated", "organization", &organizationID, map[string]string{"name": name})
	return org, nil
}

func (s *Services) DeleteOrganization(ctx context.Context, actor domain.User, organizationID uuid.UUID) error {
	billable, err := s.repo.CountActiveBillableResources(ctx, organizationID)
	if err != nil {
		return err
	}
	if billable > 0 {
		return ErrBillableResources
	}
	if err := s.repo.DeleteOrganization(ctx, organizationID); err != nil {
		return err
	}
	return nil
}

func (s *Services) ListRoles(ctx context.Context, organizationID uuid.UUID) ([]domain.Role, error) {
	return s.repo.ListRoles(ctx, organizationID)
}

func (s *Services) RequirePermission(ctx context.Context, userID, organizationID uuid.UUID, permission string) error {
	ok, err := s.repo.HasPermission(ctx, userID, organizationID, permission)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *Services) RequirePlatformPermission(ctx context.Context, userID uuid.UUID, permission string) error {
	ok, err := s.repo.HasPlatformPermission(ctx, userID, permission)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *Services) GetPlatformSession(ctx context.Context, userID uuid.UUID) (store.PlatformSession, error) {
	return s.repo.GetPlatformSession(ctx, userID)
}

func (s *Services) AddAdminAuditLog(ctx context.Context, actor domain.User, action, targetType, targetID, reason string, metadata []byte) error {
	if strings.TrimSpace(reason) == "" {
		return ErrInvalidInput
	}
	return s.repo.AddAdminAuditLog(ctx, actor.ID, action, targetType, targetID, reason, metadata)
}

func (s *Services) ListAdminUsers(ctx context.Context) ([]store.AdminUserListItem, error) {
	return s.repo.ListAdminUsers(ctx)
}

func (s *Services) ListAdminUserOrganizations(ctx context.Context, userID uuid.UUID) ([]store.AdminOrganizationListItem, error) {
	return s.repo.ListAdminUserOrganizations(ctx, userID)
}

func (s *Services) ListAdminOrganizations(ctx context.Context) ([]store.AdminOrganizationListItem, error) {
	return s.repo.ListAdminOrganizations(ctx)
}

func (s *Services) ListAdminBillingAccounts(ctx context.Context) ([]store.AdminBillingAccountListItem, error) {
	return s.repo.ListAdminBillingAccounts(ctx)
}

func (s *Services) ListAdminServers(ctx context.Context) ([]store.AdminServerListItem, error) {
	return s.repo.ListAdminServers(ctx)
}

func (s *Services) CreateAdminServer(ctx context.Context, params store.CreateAdminServerParams) (store.AdminServerListItem, error) {
	return s.repo.CreateAdminServer(ctx, params)
}

func (s *Services) AdminAssignServer(ctx context.Context, serverID, organizationID uuid.UUID) error {
	if serverID == uuid.Nil || organizationID == uuid.Nil {
		return ErrInvalidInput
	}
	return s.repo.AdminAssignServer(ctx, serverID, organizationID)
}

func (s *Services) AdminReleaseServer(ctx context.Context, serverID uuid.UUID) error {
	if serverID == uuid.Nil {
		return ErrInvalidInput
	}
	return s.repo.AdminReleaseServer(ctx, serverID)
}

func (s *Services) AdminRetireServer(ctx context.Context, serverID uuid.UUID) error {
	if serverID == uuid.Nil {
		return ErrInvalidInput
	}
	return s.repo.AdminRetireServer(ctx, serverID)
}

func (s *Services) ListHardwareOptions(ctx context.Context) ([]store.ServerCatalogHardwareOption, error) {
	return s.repo.ListHardwareOptions(ctx)
}

func (s *Services) CreateHardwareOption(ctx context.Context, params store.CreateHardwareOptionParams) (store.ServerCatalogHardwareOption, error) {
	if strings.TrimSpace(params.OptionType) == "" || strings.TrimSpace(params.Label) == "" {
		return store.ServerCatalogHardwareOption{}, ErrInvalidInput
	}
	return s.repo.CreateHardwareOption(ctx, params)
}

func (s *Services) ListHardwareFulfillmentOrders(ctx context.Context) ([]store.HardwareFulfillmentOrder, error) {
	return s.repo.ListHardwareFulfillmentOrders(ctx)
}

func (s *Services) MarkHardwareFulfillmentReady(ctx context.Context, orderID uuid.UUID) (store.HardwareFulfillmentOrder, error) {
	if orderID == uuid.Nil {
		return store.HardwareFulfillmentOrder{}, ErrInvalidInput
	}
	return s.repo.MarkHardwareFulfillmentReady(ctx, orderID)
}

func (s *Services) ListAdminRacks(ctx context.Context) ([]store.AdminRackListItem, error) {
	return s.repo.ListAdminRacks(ctx)
}

func (s *Services) ListAdminLocations(ctx context.Context) ([]store.AdminLocationListItem, error) {
	return s.repo.ListAdminLocations(ctx)
}

func (s *Services) ListAdminCPUProfiles(ctx context.Context) ([]store.AdminCPUProfileListItem, error) {
	return s.repo.ListAdminCPUProfiles(ctx)
}

func (s *Services) ListAdminServerFamilies(ctx context.Context) ([]store.AdminServerFamilyListItem, error) {
	return s.repo.ListAdminServerFamilies(ctx)
}

func (s *Services) ListAdminSwitches(ctx context.Context) ([]store.AdminSwitchListItem, error) {
	return s.repo.ListAdminSwitches(ctx)
}

func (s *Services) ListAdminEdgeRouters(ctx context.Context) ([]store.AdminEdgeRouterListItem, error) {
	return s.repo.ListAdminEdgeRouters(ctx)
}

func (s *Services) ListAdminServerNetworkInterfaces(ctx context.Context) ([]store.AdminServerNetworkInterfaceListItem, error) {
	return s.repo.ListAdminServerNetworkInterfaces(ctx)
}

func (s *Services) ListAdminHypervisors(ctx context.Context) ([]store.AdminHypervisorListItem, error) {
	return s.repo.ListAdminHypervisors(ctx)
}

func (s *Services) ListAdminHypervisorVMs(ctx context.Context, hypervisorID string) ([]store.AdminHypervisorVMListItem, error) {
	if strings.TrimSpace(hypervisorID) == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.ListAdminHypervisorVMs(ctx, hypervisorID)
}

func (s *Services) ListAdminOSImages(ctx context.Context) ([]store.AdminOSImageListItem, error) {
	return s.repo.ListAdminOSImages(ctx)
}

func (s *Services) ListAdminProvisioningJobs(ctx context.Context) ([]store.AdminProvisioningJobListItem, error) {
	return s.repo.ListAdminProvisioningJobs(ctx)
}

func (s *Services) GetProvisioningServerInventory(ctx context.Context, organizationID, serverID uuid.UUID) (store.ProvisioningServerInventory, error) {
	return s.repo.GetProvisioningServerInventory(ctx, organizationID, serverID)
}

func (s *Services) ListAdminAuditEvents(ctx context.Context) ([]store.AdminAuditEventListItem, error) {
	return s.repo.ListAdminAuditEvents(ctx)
}

func (s *Services) ListMembers(ctx context.Context, organizationID uuid.UUID) ([]domain.Member, error) {
	return s.repo.ListMembers(ctx, organizationID)
}

func (s *Services) UpdateMemberRole(ctx context.Context, actor domain.User, organizationID, targetUserID, newRoleID uuid.UUID) (domain.Member, error) {
	members, err := s.repo.ListMembers(ctx, organizationID)
	if err != nil {
		return domain.Member{}, err
	}
	var target domain.Member
	for _, member := range members {
		if member.UserID == targetUserID {
			target = member
			break
		}
	}
	if target.UserID == uuid.Nil {
		return domain.Member{}, ErrNotFound
	}
	targetIsOwner, err := s.repo.IsOwnerRole(ctx, target.RoleID)
	if err != nil {
		return domain.Member{}, err
	}
	newIsOwner, err := s.repo.IsOwnerRole(ctx, newRoleID)
	if err != nil {
		return domain.Member{}, err
	}
	if targetIsOwner && !newIsOwner {
		owners, err := s.repo.CountActiveOwners(ctx, organizationID)
		if err != nil {
			return domain.Member{}, err
		}
		if owners <= 1 {
			return domain.Member{}, ErrLastOwner
		}
	}
	member, err := s.repo.UpdateMemberRole(ctx, organizationID, targetUserID, newRoleID)
	if err != nil {
		return domain.Member{}, err
	}
	_ = s.audit(ctx, organizationID, &actor.ID, "member.role_changed", "membership", &member.ID, map[string]string{"user_id": targetUserID.String(), "role_id": newRoleID.String()})
	return member, nil
}

func (s *Services) RemoveMember(ctx context.Context, actor domain.User, organizationID, targetUserID uuid.UUID) error {
	members, err := s.repo.ListMembers(ctx, organizationID)
	if err != nil {
		return err
	}
	var target *domain.Member
	for i := range members {
		if members[i].UserID == targetUserID {
			target = &members[i]
			break
		}
	}
	if target == nil {
		return ErrNotFound
	}
	targetIsOwner, err := s.repo.IsOwnerRole(ctx, target.RoleID)
	if err != nil {
		return err
	}
	if targetIsOwner {
		owners, err := s.repo.CountActiveOwners(ctx, organizationID)
		if err != nil {
			return err
		}
		if owners <= 1 {
			return ErrLastOwner
		}
	}
	if err := s.repo.RemoveMember(ctx, organizationID, targetUserID); err != nil {
		return err
	}
	return s.audit(ctx, organizationID, &actor.ID, "member.removed", "membership", &target.ID, map[string]string{"user_id": targetUserID.String()})
}

func (s *Services) InviteMember(ctx context.Context, actor domain.User, organizationID uuid.UUID, email string, roleID uuid.UUID) (domain.Invitation, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return domain.Invitation{}, ErrInvalidInput
	}
	exists, err := s.repo.IsActiveMemberEmail(ctx, organizationID, email)
	if err != nil {
		return domain.Invitation{}, err
	}
	if exists {
		return domain.Invitation{}, ErrConflict
	}
	token, err := randomToken()
	if err != nil {
		return domain.Invitation{}, err
	}
	invitation, err := s.repo.CreateInvitation(ctx, store.CreateInvitationParams{
		OrganizationID:  organizationID,
		Email:           email,
		RoleID:          roleID,
		InvitedByUserID: actor.ID,
		Token:           token,
		ExpiresInHours:  168,
	})
	if err != nil {
		return domain.Invitation{}, err
	}
	_ = s.audit(ctx, organizationID, &actor.ID, "member.invited", "invitation", &invitation.ID, map[string]string{"email": email})
	return invitation, nil
}

func (s *Services) ListInvitations(ctx context.Context, organizationID uuid.UUID) ([]domain.Invitation, error) {
	return s.repo.ListInvitations(ctx, organizationID)
}

func (s *Services) AcceptInvitation(ctx context.Context, user domain.User, token string) (domain.Invitation, error) {
	invitation, err := s.repo.GetInvitationByToken(ctx, token)
	if err != nil {
		return domain.Invitation{}, err
	}
	if invitation.Status != domain.InvitationPending {
		return domain.Invitation{}, ErrInvitationNotPending
	}
	if time.Now().After(invitation.ExpiresAt) {
		return domain.Invitation{}, ErrInvitationExpired
	}
	if !strings.EqualFold(invitation.Email, user.Email) {
		return domain.Invitation{}, ErrForbidden
	}
	return s.repo.AcceptInvitation(ctx, token, user.ID)
}

func (s *Services) RevokeInvitation(ctx context.Context, actor domain.User, organizationID, invitationID uuid.UUID) (domain.Invitation, error) {
	invitation, err := s.repo.RevokeInvitation(ctx, organizationID, invitationID)
	if err != nil {
		return domain.Invitation{}, err
	}
	_ = s.audit(ctx, organizationID, &actor.ID, "member.invitation_revoked", "invitation", &invitation.ID, nil)
	return invitation, nil
}

func (s *Services) CreateProject(ctx context.Context, actor domain.User, organizationID uuid.UUID, name string) (domain.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Project{}, ErrInvalidInput
	}
	project, err := s.repo.CreateProject(ctx, organizationID, name, slugify(name))
	if err != nil {
		return domain.Project{}, err
	}
	_ = s.audit(ctx, organizationID, &actor.ID, "project.created", "project", &project.ID, map[string]string{"slug": project.Slug})
	return project, nil
}

func (s *Services) ListProjects(ctx context.Context, organizationID uuid.UUID) ([]domain.Project, error) {
	return s.repo.ListProjects(ctx, organizationID)
}

func (s *Services) GetProject(ctx context.Context, organizationID, projectID uuid.UUID) (domain.Project, error) {
	return s.repo.GetProject(ctx, organizationID, projectID)
}

func (s *Services) UpdateProject(ctx context.Context, actor domain.User, organizationID, projectID uuid.UUID, name string) (domain.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Project{}, ErrInvalidInput
	}
	project, err := s.repo.UpdateProject(ctx, organizationID, projectID, name, slugify(name))
	if err != nil {
		return domain.Project{}, err
	}
	_ = s.audit(ctx, organizationID, &actor.ID, "project.updated", "project", &project.ID, map[string]string{"slug": project.Slug})
	return project, nil
}

func (s *Services) DeleteProject(ctx context.Context, actor domain.User, organizationID, projectID uuid.UUID) error {
	if err := s.repo.DeleteProject(ctx, organizationID, projectID); err != nil {
		return err
	}
	return s.audit(ctx, organizationID, &actor.ID, "project.deleted", "project", &projectID, nil)
}

func (s *Services) GetBillingProfile(ctx context.Context, organizationID uuid.UUID) (domain.BillingProfile, error) {
	return s.repo.GetBillingProfile(ctx, organizationID)
}

func (s *Services) UpdateBillingProfile(ctx context.Context, actor domain.User, organizationID uuid.UUID, params store.UpdateBillingProfileParams) (domain.BillingProfile, error) {
	profile, err := s.repo.UpdateBillingProfile(ctx, organizationID, params)
	if err != nil {
		return domain.BillingProfile{}, err
	}
	_ = s.audit(ctx, organizationID, &actor.ID, "billing.updated", "billing_profile", &profile.ID, nil)
	return profile, nil
}

func (s *Services) ListPaymentMethods(ctx context.Context, organizationID uuid.UUID) ([]domain.PaymentMethod, error) {
	return s.repo.ListPaymentMethods(ctx, organizationID)
}

func (s *Services) CreatePaymentMethod(ctx context.Context, actor domain.User, method domain.PaymentMethod) (domain.PaymentMethod, error) {
	created, err := s.repo.CreatePaymentMethod(ctx, method)
	if err != nil {
		return domain.PaymentMethod{}, err
	}
	_ = s.audit(ctx, method.OrganizationID, &actor.ID, "payment_method.added", "payment_method", &created.ID, map[string]string{"brand": created.Brand, "last4": created.Last4})
	return created, nil
}

func (s *Services) DeletePaymentMethod(ctx context.Context, actor domain.User, organizationID, paymentMethodID uuid.UUID) error {
	if err := s.repo.DeletePaymentMethod(ctx, organizationID, paymentMethodID); err != nil {
		return err
	}
	return s.audit(ctx, organizationID, &actor.ID, "payment_method.removed", "payment_method", &paymentMethodID, nil)
}

func (s *Services) ListInvoices(ctx context.Context, organizationID uuid.UUID) ([]domain.Invoice, error) {
	return s.repo.ListInvoices(ctx, organizationID)
}

func (s *Services) ListAuditLog(ctx context.Context, organizationID uuid.UUID) ([]domain.AuditLogEntry, error) {
	return s.repo.ListAuditLog(ctx, organizationID)
}

func (s *Services) ResourceBelongsToOrganization(ctx context.Context, resourceType string, resourceID, organizationID uuid.UUID) (bool, error) {
	return s.repo.ResourceBelongsToOrganization(ctx, resourceType, resourceID, organizationID)
}

func (s *Services) audit(ctx context.Context, organizationID uuid.UUID, actorUserID *uuid.UUID, action, entityType string, entityID *uuid.UUID, metadata any) error {
	payload := []byte(`{}`)
	if metadata != nil {
		encoded, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		payload = encoded
	}
	return s.repo.AddAuditLog(ctx, organizationID, actorUserID, action, entityType, entityID, payload)
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, store.ErrNotFound)
}

func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict) || errors.Is(err, store.ErrConflict)
}

func IsInvalidInput(err error) bool {
	return errors.Is(err, ErrInvalidInput) || errors.Is(err, store.ErrInvalidInput)
}
