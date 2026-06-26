package store

import (
	"context"
	"encoding/json"
	"errors"

	"relay/client-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func OpenPostgres(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Close() {
	s.pool.Close()
}

func (s *PostgresStore) UpsertUser(ctx context.Context, identity AuthIdentity) (domain.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		insert into users (auth0_sub, email, name)
		values ($1, coalesce(nullif($2, ''), $1), coalesce(nullif($3, ''), coalesce(nullif($2, ''), $1)))
		on conflict (auth0_sub) do update
		set email = case when $2 <> '' then excluded.email else users.email end,
			name = case when $3 <> '' then excluded.name else users.name end,
			updated_at = now()
		returning id, auth0_sub, email, name, created_at, updated_at`,
		identity.Auth0Sub, identity.Email, identity.Name,
	)
	var user domain.User
	if err := row.Scan(&user.ID, &user.Auth0Sub, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return domain.User{}, err
	}
	if err := ensureDefaultOrganization(ctx, tx, user); err != nil {
		return domain.User{}, err
	}
	return user, tx.Commit(ctx)
}

func ensureDefaultOrganization(ctx context.Context, tx pgx.Tx, user domain.User) error {
	var hasActiveOrganization bool
	if err := tx.QueryRow(ctx, `
		select exists (
			select 1
			from organization_memberships
			where user_id = $1 and status = 'active'
		)`, user.ID).Scan(&hasActiveOrganization); err != nil {
		return err
	}
	if hasActiveOrganization {
		if err := ensureDefaultProjectsForUserOrganizations(ctx, tx, user.ID); err != nil {
			return err
		}
		return nil
	}

	var ownerRoleID uuid.UUID
	if err := tx.QueryRow(ctx, `select id from roles where organization_id is null and name = $1`, domain.RoleOwner).Scan(&ownerRoleID); err != nil {
		return err
	}

	slug := "default-" + user.ID.String()[:8]
	var org domain.Organization
	if err := tx.QueryRow(ctx, `
		insert into organizations (name, slug, created_by_user_id)
		values ('Default', $1, $2)
		returning id, name, slug, created_by_user_id, created_at, updated_at`,
		slug, user.ID,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedByUserID, &org.CreatedAt, &org.UpdatedAt); err != nil {
		return mapConstraint(err)
	}

	if _, err := tx.Exec(ctx, `
		insert into organization_memberships (organization_id, user_id, role_id, status, joined_at)
		values ($1, $2, $3, 'active', now())`, org.ID, user.ID, ownerRoleID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		insert into billing_profiles (organization_id, billing_email, company_name)
		values ($1, $2, $3)`, org.ID, user.Email, org.Name); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		insert into billing_accounts (organization_id, billing_email, currency, status, payment_terms)
		values ($1, $2, 'usd', 'active', 'prepaid')`, org.ID, user.Email); err != nil {
		return err
	}

	if err := ensureDefaultProject(ctx, tx, org.ID); err != nil {
		return err
	}

	meta, _ := json.Marshal(map[string]any{
		"slug":                 org.Slug,
		"default_organization": true,
		"source":               "user_upsert",
	})
	_, err := tx.Exec(ctx, `
		insert into audit_log (organization_id, actor_user_id, action, entity_type, entity_id, metadata)
		values ($1, $2, 'organization.created', 'organization', $3, $4)`,
		org.ID, user.ID, org.ID, meta,
	)
	return err
}

func ensureDefaultProjectsForUserOrganizations(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	rows, err := tx.Query(ctx, `
		select organization_id
		from organization_memberships
		where user_id = $1 and status = 'active'`, userID)
	if err != nil {
		return err
	}

	var organizationIDs []uuid.UUID
	for rows.Next() {
		var organizationID uuid.UUID
		if err := rows.Scan(&organizationID); err != nil {
			rows.Close()
			return err
		}
		organizationIDs = append(organizationIDs, organizationID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, organizationID := range organizationIDs {
		if err := ensureDefaultProject(ctx, tx, organizationID); err != nil {
			return err
		}
	}
	return nil
}

func ensureDefaultProject(ctx context.Context, tx pgx.Tx, organizationID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		insert into projects (organization_id, name, slug)
		values ($1, 'Default', 'default')
		on conflict (organization_id, slug) do nothing`, organizationID)
	return err
}

func (s *PostgresStore) GetUserByAuth0Sub(ctx context.Context, auth0Sub string) (domain.User, error) {
	row := s.pool.QueryRow(ctx, `
		select id, auth0_sub, email, name, created_at, updated_at
		from users
		where auth0_sub = $1`, auth0Sub)
	var user domain.User
	err := row.Scan(&user.ID, &user.Auth0Sub, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt)
	return user, mapNoRows(err)
}

func (s *PostgresStore) CreateOrganization(ctx context.Context, params CreateOrganizationParams) (domain.Organization, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Organization{}, err
	}
	defer tx.Rollback(ctx)

	var org domain.Organization
	err = tx.QueryRow(ctx, `
		insert into organizations (name, slug, created_by_user_id)
		values ($1, $2, $3)
		returning id, name, slug, created_by_user_id, created_at, updated_at`,
		params.Name, params.Slug, params.CreatedByUserID,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedByUserID, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return domain.Organization{}, mapConstraint(err)
	}

	var ownerRoleID uuid.UUID
	if err := tx.QueryRow(ctx, `select id from roles where organization_id is null and name = $1`, domain.RoleOwner).Scan(&ownerRoleID); err != nil {
		return domain.Organization{}, err
	}

	_, err = tx.Exec(ctx, `
		insert into organization_memberships (organization_id, user_id, role_id, status, joined_at)
		values ($1, $2, $3, 'active', now())`, org.ID, params.CreatedByUserID, ownerRoleID)
	if err != nil {
		return domain.Organization{}, err
	}

	_, err = tx.Exec(ctx, `
		insert into billing_profiles (organization_id, billing_email, company_name)
		values ($1, $2, $3)`, org.ID, params.BillingEmail, params.Name)
	if err != nil {
		return domain.Organization{}, err
	}

	_, err = tx.Exec(ctx, `
		insert into billing_accounts (organization_id, billing_email, currency, status, payment_terms)
		values ($1, $2, 'usd', 'active', 'prepaid')`, org.ID, params.BillingEmail)
	if err != nil {
		return domain.Organization{}, err
	}

	if err := ensureDefaultProject(ctx, tx, org.ID); err != nil {
		return domain.Organization{}, err
	}

	meta, _ := json.Marshal(map[string]string{"slug": org.Slug})
	_, err = tx.Exec(ctx, `
		insert into audit_log (organization_id, actor_user_id, action, entity_type, entity_id, metadata)
		values ($1, $2, 'organization.created', 'organization', $3, $4)`,
		org.ID, params.CreatedByUserID, org.ID, meta,
	)
	if err != nil {
		return domain.Organization{}, err
	}

	return org, tx.Commit(ctx)
}

func (s *PostgresStore) ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]domain.Organization, error) {
	rows, err := s.pool.Query(ctx, `
		select o.id, o.name, o.slug, o.created_by_user_id, o.created_at, o.updated_at
		from organizations o
		join organization_memberships m on m.organization_id = o.id
		where m.user_id = $1 and m.status = 'active'
		order by o.created_at desc`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrganizations(rows)
}

func (s *PostgresStore) GetOrganizationForUser(ctx context.Context, organizationID, userID uuid.UUID) (domain.Organization, error) {
	row := s.pool.QueryRow(ctx, `
		select o.id, o.name, o.slug, o.created_by_user_id, o.created_at, o.updated_at
		from organizations o
		join organization_memberships m on m.organization_id = o.id
		where o.id = $1 and m.user_id = $2 and m.status = 'active'`, organizationID, userID)
	var org domain.Organization
	err := row.Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedByUserID, &org.CreatedAt, &org.UpdatedAt)
	return org, mapNoRows(err)
}

func (s *PostgresStore) GetOrganizationByID(ctx context.Context, organizationID uuid.UUID) (domain.Organization, error) {
	row := s.pool.QueryRow(ctx, `
		select id, name, slug, created_by_user_id, created_at, updated_at
		from organizations
		where id = $1`, organizationID)
	var org domain.Organization
	err := row.Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedByUserID, &org.CreatedAt, &org.UpdatedAt)
	return org, mapNoRows(err)
}

func (s *PostgresStore) UpdateOrganization(ctx context.Context, organizationID uuid.UUID, name, slug string) (domain.Organization, error) {
	row := s.pool.QueryRow(ctx, `
		update organizations
		set name = $2, slug = $3, updated_at = now()
		where id = $1
		returning id, name, slug, created_by_user_id, created_at, updated_at`, organizationID, name, slug)
	var org domain.Organization
	err := row.Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedByUserID, &org.CreatedAt, &org.UpdatedAt)
	return org, mapConstraint(mapNoRows(err))
}

func (s *PostgresStore) DeleteOrganization(ctx context.Context, organizationID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `delete from organizations where id = $1`, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PostgresStore) CountActiveBillableResources(ctx context.Context, organizationID uuid.UUID) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx, `
		select count(*)
		from billable_services
		where organization_id = $1 and status in ('provisioning', 'active', 'suspended')`, organizationID).Scan(&count)
	return count, err
}

func (s *PostgresStore) GetSystemRoleByName(ctx context.Context, name string) (domain.Role, error) {
	row := s.pool.QueryRow(ctx, `
		select id, organization_id, name, is_system_role, created_at, updated_at
		from roles
		where organization_id is null and name = $1`, name)
	return scanRole(row)
}

func (s *PostgresStore) GetRole(ctx context.Context, roleID uuid.UUID) (domain.Role, error) {
	row := s.pool.QueryRow(ctx, `
		select id, organization_id, name, is_system_role, created_at, updated_at
		from roles
		where id = $1`, roleID)
	return scanRole(row)
}

func (s *PostgresStore) ListRoles(ctx context.Context, organizationID uuid.UUID) ([]domain.Role, error) {
	rows, err := s.pool.Query(ctx, `
		select id, organization_id, name, is_system_role, created_at, updated_at
		from roles
		where organization_id is null or organization_id = $1
		order by is_system_role desc, name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []domain.Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (s *PostgresStore) HasPermission(ctx context.Context, userID, organizationID uuid.UUID, permission string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		select exists (
			select 1
			from organization_memberships m
			join role_permissions rp on rp.role_id = m.role_id
			where m.organization_id = $1
				and m.user_id = $2
				and m.status = 'active'
				and rp.permission = $3
		)`, organizationID, userID, permission).Scan(&ok)
	return ok, err
}

func (s *PostgresStore) ListMembers(ctx context.Context, organizationID uuid.UUID) ([]domain.Member, error) {
	rows, err := s.pool.Query(ctx, `
		select m.id, m.organization_id, m.user_id, m.role_id, r.name, m.status, m.joined_at,
			u.email, u.name, m.created_at, m.updated_at
		from organization_memberships m
		join users u on u.id = m.user_id
		join roles r on r.id = m.role_id
		where m.organization_id = $1 and m.status <> 'removed'
		order by u.email`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []domain.Member
	for rows.Next() {
		var member domain.Member
		if err := rows.Scan(&member.ID, &member.OrganizationID, &member.UserID, &member.RoleID, &member.RoleName, &member.Status, &member.JoinedAt, &member.Email, &member.Name, &member.CreatedAt, &member.UpdatedAt); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *PostgresStore) UpdateMemberRole(ctx context.Context, organizationID, targetUserID, roleID uuid.UUID) (domain.Member, error) {
	row := s.pool.QueryRow(ctx, `
		update organization_memberships
		set role_id = $3, updated_at = now()
		where organization_id = $1 and user_id = $2 and status = 'active'
		returning id`, organizationID, targetUserID, roleID)
	var ignored uuid.UUID
	if err := row.Scan(&ignored); err != nil {
		return domain.Member{}, mapNoRows(err)
	}
	members, err := s.ListMembers(ctx, organizationID)
	if err != nil {
		return domain.Member{}, err
	}
	for _, member := range members {
		if member.UserID == targetUserID {
			return member, nil
		}
	}
	return domain.Member{}, pgx.ErrNoRows
}

func (s *PostgresStore) RemoveMember(ctx context.Context, organizationID, targetUserID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		update organization_memberships
		set status = 'removed', updated_at = now()
		where organization_id = $1 and user_id = $2 and status = 'active'`, organizationID, targetUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PostgresStore) CountActiveOwners(ctx context.Context, organizationID uuid.UUID) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx, `
		select count(*)
		from organization_memberships m
		join roles r on r.id = m.role_id
		where m.organization_id = $1 and m.status = 'active' and r.name = $2`, organizationID, domain.RoleOwner).Scan(&count)
	return count, err
}

func (s *PostgresStore) IsOwnerRole(ctx context.Context, roleID uuid.UUID) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists(select 1 from roles where id = $1 and name = $2)`, roleID, domain.RoleOwner).Scan(&ok)
	return ok, err
}

func (s *PostgresStore) IsActiveMemberEmail(ctx context.Context, organizationID uuid.UUID, email string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		select exists (
			select 1 from organization_memberships m
			join users u on u.id = m.user_id
			where m.organization_id = $1 and lower(u.email) = lower($2) and m.status in ('active', 'invited')
		)`, organizationID, email).Scan(&ok)
	return ok, err
}

func (s *PostgresStore) CreateInvitation(ctx context.Context, params CreateInvitationParams) (domain.Invitation, error) {
	row := s.pool.QueryRow(ctx, `
		insert into invitations (organization_id, email, role_id, invited_by_user_id, token, status, expires_at)
		values ($1, lower($2), $3, $4, $5, 'pending', now() + make_interval(hours => $6))
		returning id, organization_id, email, role_id, invited_by_user_id, token, status, expires_at, accepted_at, created_at, updated_at`,
		params.OrganizationID, params.Email, params.RoleID, params.InvitedByUserID, params.Token, params.ExpiresInHours)
	return s.scanInvitation(row)
}

func (s *PostgresStore) ListInvitations(ctx context.Context, organizationID uuid.UUID) ([]domain.Invitation, error) {
	rows, err := s.pool.Query(ctx, `
		select i.id, i.organization_id, i.email, i.role_id, i.invited_by_user_id, i.token, i.status, i.expires_at, i.accepted_at, i.created_at, i.updated_at, r.name
		from invitations i
		join roles r on r.id = i.role_id
		where i.organization_id = $1
		order by i.created_at desc`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var invitations []domain.Invitation
	for rows.Next() {
		invitation, err := scanInvitationRows(rows)
		if err != nil {
			return nil, err
		}
		invitation.Token = ""
		invitations = append(invitations, invitation)
	}
	return invitations, rows.Err()
}

func (s *PostgresStore) GetInvitationByToken(ctx context.Context, token string) (domain.Invitation, error) {
	row := s.pool.QueryRow(ctx, `
		select i.id, i.organization_id, i.email, i.role_id, i.invited_by_user_id, i.token, i.status, i.expires_at, i.accepted_at, i.created_at, i.updated_at, r.name
		from invitations i
		join roles r on r.id = i.role_id
		where i.token = $1`, token)
	return scanInvitationRows(row)
}

func (s *PostgresStore) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) (domain.Invitation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Invitation{}, err
	}
	defer tx.Rollback(ctx)

	invitation, err := scanInvitationRows(tx.QueryRow(ctx, `
		select i.id, i.organization_id, i.email, i.role_id, i.invited_by_user_id, i.token, i.status, i.expires_at, i.accepted_at, i.created_at, i.updated_at, r.name
		from invitations i
		join roles r on r.id = i.role_id
		where i.token = $1
		for update`, token))
	if err != nil {
		return domain.Invitation{}, mapNoRows(err)
	}

	err = tx.QueryRow(ctx, `
		update invitations
		set status = 'accepted', accepted_at = now(), updated_at = now()
		where id = $1
		returning accepted_at, updated_at`, invitation.ID).Scan(&invitation.AcceptedAt, &invitation.UpdatedAt)
	if err != nil {
		return domain.Invitation{}, err
	}
	invitation.Status = domain.InvitationAccepted

	_, err = tx.Exec(ctx, `
		insert into organization_memberships (organization_id, user_id, role_id, status, joined_at)
		values ($1, $2, $3, 'active', now())
		on conflict (organization_id, user_id) do update
		set role_id = excluded.role_id, status = 'active', joined_at = coalesce(organization_memberships.joined_at, now()), updated_at = now()`,
		invitation.OrganizationID, userID, invitation.RoleID)
	if err != nil {
		return domain.Invitation{}, err
	}

	meta, _ := json.Marshal(map[string]string{"email": invitation.Email})
	_, err = tx.Exec(ctx, `
		insert into audit_log (organization_id, actor_user_id, action, entity_type, entity_id, metadata)
		values ($1, $2, 'member.joined', 'membership', $2, $3)`,
		invitation.OrganizationID, userID, meta)
	if err != nil {
		return domain.Invitation{}, err
	}

	return invitation, tx.Commit(ctx)
}

func (s *PostgresStore) RevokeInvitation(ctx context.Context, organizationID, invitationID uuid.UUID) (domain.Invitation, error) {
	row := s.pool.QueryRow(ctx, `
		update invitations
		set status = 'revoked', updated_at = now()
		where organization_id = $1 and id = $2 and status = 'pending'
		returning id, organization_id, email, role_id, invited_by_user_id, token, status, expires_at, accepted_at, created_at, updated_at`,
		organizationID, invitationID)
	return s.scanInvitation(row)
}

func (s *PostgresStore) CreateProject(ctx context.Context, organizationID uuid.UUID, name, slug string) (domain.Project, error) {
	row := s.pool.QueryRow(ctx, `
		insert into projects (organization_id, name, slug)
		values ($1, $2, $3)
		returning id, organization_id, name, slug, created_at, updated_at`, organizationID, name, slug)
	return scanProject(row)
}

func (s *PostgresStore) ListProjects(ctx context.Context, organizationID uuid.UUID) ([]domain.Project, error) {
	rows, err := s.pool.Query(ctx, `
		select id, organization_id, name, slug, created_at, updated_at
		from projects
		where organization_id = $1
		order by name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []domain.Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s *PostgresStore) GetProject(ctx context.Context, organizationID, projectID uuid.UUID) (domain.Project, error) {
	row := s.pool.QueryRow(ctx, `
		select id, organization_id, name, slug, created_at, updated_at
		from projects
		where organization_id = $1 and id = $2`, organizationID, projectID)
	return scanProject(row)
}

func (s *PostgresStore) UpdateProject(ctx context.Context, organizationID, projectID uuid.UUID, name, slug string) (domain.Project, error) {
	row := s.pool.QueryRow(ctx, `
		update projects
		set name = $3, slug = $4, updated_at = now()
		where organization_id = $1 and id = $2
		returning id, organization_id, name, slug, created_at, updated_at`, organizationID, projectID, name, slug)
	return scanProject(row)
}

func (s *PostgresStore) DeleteProject(ctx context.Context, organizationID, projectID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `delete from projects where organization_id = $1 and id = $2`, organizationID, projectID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PostgresStore) GetBillingProfile(ctx context.Context, organizationID uuid.UUID) (domain.BillingProfile, error) {
	row := s.pool.QueryRow(ctx, `
		select id, organization_id, stripe_customer_id, billing_email, company_name, tax_id, line1, line2, city, state, postal_code, country, created_at, updated_at
		from billing_profiles
		where organization_id = $1`, organizationID)
	return scanBillingProfile(row)
}

func (s *PostgresStore) UpdateBillingProfile(ctx context.Context, organizationID uuid.UUID, params UpdateBillingProfileParams) (domain.BillingProfile, error) {
	// TODO: Mirror billing profile updates to Stripe Customer once Stripe is wired.
	row := s.pool.QueryRow(ctx, `
		update billing_profiles
		set billing_email = $2, company_name = $3, tax_id = $4, line1 = $5, line2 = $6, city = $7, state = $8, postal_code = $9, country = $10, updated_at = now()
		where organization_id = $1
		returning id, organization_id, stripe_customer_id, billing_email, company_name, tax_id, line1, line2, city, state, postal_code, country, created_at, updated_at`,
		organizationID, params.BillingEmail, params.CompanyName, params.TaxID, params.Line1, params.Line2, params.City, params.State, params.PostalCode, params.Country)
	return scanBillingProfile(row)
}

func (s *PostgresStore) ListPaymentMethods(ctx context.Context, organizationID uuid.UUID) ([]domain.PaymentMethod, error) {
	rows, err := s.pool.Query(ctx, `
		select id, organization_id, stripe_payment_method_id, brand, last4, exp_month, exp_year, is_default, created_at, updated_at
		from payment_methods
		where organization_id = $1
		order by is_default desc, created_at desc`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var methods []domain.PaymentMethod
	for rows.Next() {
		var method domain.PaymentMethod
		if err := rows.Scan(&method.ID, &method.OrganizationID, &method.StripePaymentMethodID, &method.Brand, &method.Last4, &method.ExpMonth, &method.ExpYear, &method.IsDefault, &method.CreatedAt, &method.UpdatedAt); err != nil {
			return nil, err
		}
		methods = append(methods, method)
	}
	return methods, rows.Err()
}

func (s *PostgresStore) CreatePaymentMethod(ctx context.Context, method domain.PaymentMethod) (domain.PaymentMethod, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PaymentMethod{}, err
	}
	defer tx.Rollback(ctx)

	if method.IsDefault {
		if _, err := tx.Exec(ctx, `
			update payment_methods
			set is_default = false,
				updated_at = now()
			where organization_id = $1`, method.OrganizationID); err != nil {
			return domain.PaymentMethod{}, err
		}
	}

	row := tx.QueryRow(ctx, `
		insert into payment_methods (organization_id, stripe_payment_method_id, brand, last4, exp_month, exp_year, is_default)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (stripe_payment_method_id) do update
		set brand = excluded.brand,
			last4 = excluded.last4,
			exp_month = excluded.exp_month,
			exp_year = excluded.exp_year,
			is_default = payment_methods.is_default or excluded.is_default,
			updated_at = now()
		returning id, organization_id, stripe_payment_method_id, brand, last4, exp_month, exp_year, is_default, created_at, updated_at`,
		method.OrganizationID, method.StripePaymentMethodID, method.Brand, method.Last4, method.ExpMonth, method.ExpYear, method.IsDefault)
	if err := row.Scan(&method.ID, &method.OrganizationID, &method.StripePaymentMethodID, &method.Brand, &method.Last4, &method.ExpMonth, &method.ExpYear, &method.IsDefault, &method.CreatedAt, &method.UpdatedAt); err != nil {
		return domain.PaymentMethod{}, mapConstraint(err)
	}
	return method, tx.Commit(ctx)
}

func (s *PostgresStore) DeletePaymentMethod(ctx context.Context, organizationID, paymentMethodID uuid.UUID) error {
	// TODO: Detach the Stripe payment method before deleting this local record.
	tag, err := s.pool.Exec(ctx, `delete from payment_methods where organization_id = $1 and id = $2`, organizationID, paymentMethodID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PostgresStore) ListInvoices(ctx context.Context, organizationID uuid.UUID) ([]domain.Invoice, error) {
	rows, err := s.pool.Query(ctx, `
		select id, organization_id, stripe_invoice_id, status, amount_due, amount_paid, period_start, period_end, created_at, updated_at
		from invoices
		where organization_id = $1
		order by period_start desc`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var invoices []domain.Invoice
	for rows.Next() {
		var invoice domain.Invoice
		if err := rows.Scan(&invoice.ID, &invoice.OrganizationID, &invoice.StripeInvoiceID, &invoice.Status, &invoice.AmountDue, &invoice.AmountPaid, &invoice.PeriodStart, &invoice.PeriodEnd, &invoice.CreatedAt, &invoice.UpdatedAt); err != nil {
			return nil, err
		}
		invoices = append(invoices, invoice)
	}
	return invoices, rows.Err()
}

func (s *PostgresStore) AddAuditLog(ctx context.Context, organizationID uuid.UUID, actorUserID *uuid.UUID, action, entityType string, entityID *uuid.UUID, metadata []byte) error {
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	_, err := s.pool.Exec(ctx, `
		insert into audit_log (organization_id, actor_user_id, action, entity_type, entity_id, metadata)
		values ($1, $2, $3, $4, $5, $6)`, organizationID, actorUserID, action, entityType, entityID, metadata)
	return err
}

func (s *PostgresStore) ListAuditLog(ctx context.Context, organizationID uuid.UUID) ([]domain.AuditLogEntry, error) {
	rows, err := s.pool.Query(ctx, `
		select id, organization_id, actor_user_id, action, entity_type, entity_id, metadata, created_at
		from audit_log
		where organization_id = $1
		order by created_at desc
		limit 250`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []domain.AuditLogEntry
	for rows.Next() {
		var entry domain.AuditLogEntry
		if err := rows.Scan(&entry.ID, &entry.OrganizationID, &entry.ActorUserID, &entry.Action, &entry.EntityType, &entry.EntityID, &entry.Metadata, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *PostgresStore) ResourceBelongsToOrganization(ctx context.Context, resourceType string, resourceID, organizationID uuid.UUID) (bool, error) {
	switch resourceType {
	case "projects":
		var ok bool
		err := s.pool.QueryRow(ctx, `select exists(select 1 from projects where id = $1 and organization_id = $2)`, resourceID, organizationID).Scan(&ok)
		return ok, err
	default:
		// TODO: Add servers, networks, SSH keys, API keys, and clusters as those resource tables land.
		return false, nil
	}
}

func (s *PostgresStore) scanInvitation(row pgx.Row) (domain.Invitation, error) {
	invitation, err := scanInvitationBase(row)
	if err != nil {
		return domain.Invitation{}, mapConstraint(mapNoRows(err))
	}
	role, err := s.GetRole(context.Background(), invitation.RoleID)
	if err == nil {
		invitation.RoleName = role.Name
	}
	return invitation, nil
}

func scanInvitationBase(row pgx.Row) (domain.Invitation, error) {
	var invitation domain.Invitation
	err := row.Scan(&invitation.ID, &invitation.OrganizationID, &invitation.Email, &invitation.RoleID, &invitation.InvitedByUserID, &invitation.Token, &invitation.Status, &invitation.ExpiresAt, &invitation.AcceptedAt, &invitation.CreatedAt, &invitation.UpdatedAt)
	return invitation, err
}

func scanInvitationRows(row pgx.Row) (domain.Invitation, error) {
	var invitation domain.Invitation
	err := row.Scan(&invitation.ID, &invitation.OrganizationID, &invitation.Email, &invitation.RoleID, &invitation.InvitedByUserID, &invitation.Token, &invitation.Status, &invitation.ExpiresAt, &invitation.AcceptedAt, &invitation.CreatedAt, &invitation.UpdatedAt, &invitation.RoleName)
	return invitation, mapNoRows(err)
}

func scanRole(row pgx.Row) (domain.Role, error) {
	var role domain.Role
	err := row.Scan(&role.ID, &role.OrganizationID, &role.Name, &role.IsSystemRole, &role.CreatedAt, &role.UpdatedAt)
	return role, mapNoRows(err)
}

func scanProject(row pgx.Row) (domain.Project, error) {
	var project domain.Project
	err := row.Scan(&project.ID, &project.OrganizationID, &project.Name, &project.Slug, &project.CreatedAt, &project.UpdatedAt)
	return project, mapConstraint(mapNoRows(err))
}

func scanBillingProfile(row pgx.Row) (domain.BillingProfile, error) {
	var profile domain.BillingProfile
	err := row.Scan(&profile.ID, &profile.OrganizationID, &profile.StripeCustomerID, &profile.BillingEmail, &profile.CompanyName, &profile.TaxID, &profile.Line1, &profile.Line2, &profile.City, &profile.State, &profile.PostalCode, &profile.Country, &profile.CreatedAt, &profile.UpdatedAt)
	return profile, mapNoRows(err)
}

func scanOrganizations(rows pgx.Rows) ([]domain.Organization, error) {
	var orgs []domain.Organization
	for rows.Next() {
		var org domain.Organization
		if err := rows.Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedByUserID, &org.CreatedAt, &org.UpdatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, org)
	}
	return orgs, rows.Err()
}

func mapNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func mapConstraint(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalidInput = errors.New("invalid input")
)
