// Package developer orchestrates a signed-in developer's organizations,
// applications and API keys.
package developer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

type Service struct {
	orgs    OrganizationRepository
	apps    ApplicationRepository
	keys    KeyRepository
	invites InvitationRepository
	audit   AuditRepository
	now     func() time.Time
}

type OrganizationRepository interface {
	Create(context.Context, identity.Organization) error
	ByIDForOwner(context.Context, string, string) (*identity.Organization, error)
	ByIDForAccount(context.Context, string, string) (*identity.Organization, error)
	ListByAccount(context.Context, string) ([]identity.Organization, error)
	AddMember(context.Context, string, identity.OrganizationMember) error
	TransferOwnership(context.Context, string, string, string, time.Time) error
}
type ApplicationRepository interface {
	Create(context.Context, identity.Application) error
	ByID(context.Context, string, string) (*identity.Application, error)
	ListByOrganization(context.Context, string) ([]identity.Application, error)
}
type KeyRepository interface {
	Create(context.Context, identity.APIKey) error
	Revoke(context.Context, string, time.Time) error
	ByIDForApplication(context.Context, string, string) (*identity.APIKey, error)
	ListByApplication(context.Context, string) ([]identity.APIKey, error)
}
type AuditRepository interface {
	Append(context.Context, audit.Entry) (audit.Entry, error)
}

type InvitationRepository interface {
	Create(context.Context, identity.OrganizationInvitation) error
	ListPending(context.Context, string) ([]identity.OrganizationInvitation, error)
	ByTokenHash(context.Context, string) (*identity.OrganizationInvitation, error)
	Accept(context.Context, string, time.Time) error
	Revoke(context.Context, string, string) error
}

func NewService(orgs OrganizationRepository, apps ApplicationRepository, keys KeyRepository, invites InvitationRepository, auditRepo AuditRepository) *Service {
	return &Service{orgs: orgs, apps: apps, keys: keys, invites: invites, audit: auditRepo, now: time.Now}
}

func (s *Service) CreateOrganization(ctx context.Context, accountID, email, name, requestID, ip string) (identity.Organization, error) {
	id, err := identity.NewID("org")
	if err != nil {
		return identity.Organization{}, internal(err)
	}
	now := s.now().UTC()
	org := identity.Organization{ID: id, Name: strings.TrimSpace(name), OwnerID: accountID, Members: []identity.OrganizationMember{{AccountID: accountID, Email: strings.ToLower(strings.TrimSpace(email)), Role: identity.OrganizationOwner, JoinedAt: now}}, CreatedAt: now}
	if err := org.Validate(); err != nil {
		return identity.Organization{}, invalid(err)
	}
	if err := s.orgs.Create(ctx, org); err != nil {
		return identity.Organization{}, internal(err)
	}
	s.record(ctx, accountID, requestID, ip, audit.ActionOrganizationCreated, "organization", org.ID, org.Name, nil, map[string]any{"name": org.Name})
	return org, nil
}

func (s *Service) Organizations(ctx context.Context, accountID string) ([]identity.Organization, error) {
	out, err := s.orgs.ListByAccount(ctx, accountID)
	if err != nil {
		return nil, internal(err)
	}
	return out, nil
}

type CreateApplicationInput struct {
	Name, Description string
	Environments      []identity.Environment
	Domains           []string
	CallbackURL       string
}

func (s *Service) CreateApplication(ctx context.Context, accountID, orgID string, in CreateApplicationInput, requestID, ip string) (identity.Application, error) {
	if _, err := s.requireRole(ctx, accountID, orgID, identity.OrganizationOwner, identity.OrganizationAdmin, identity.OrganizationMemberRole); err != nil {
		return identity.Application{}, err
	}
	id, err := identity.NewID("app")
	if err != nil {
		return identity.Application{}, internal(err)
	}
	envs := in.Environments
	if len(envs) == 0 {
		envs = []identity.Environment{identity.EnvTest, identity.EnvLive}
	}
	app := identity.Application{ID: id, OrganizationID: orgID, Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description), Environments: envs, Domains: cleanStrings(in.Domains), CallbackURL: strings.TrimSpace(in.CallbackURL), Plan: "free", CreatedAt: s.now().UTC()}
	if err := app.Validate(); err != nil {
		return identity.Application{}, invalid(err)
	}
	if err := s.apps.Create(ctx, app); err != nil {
		return identity.Application{}, internal(err)
	}
	s.record(ctx, accountID, requestID, ip, audit.ActionApplicationCreated, "application", app.ID, app.Name, nil, map[string]any{"organizationId": orgID, "name": app.Name})
	return app, nil
}

func (s *Service) Applications(ctx context.Context, accountID, orgID string) ([]identity.Application, error) {
	if _, err := s.memberOrg(ctx, accountID, orgID); err != nil {
		return nil, err
	}
	out, err := s.apps.ListByOrganization(ctx, orgID)
	if err != nil {
		return nil, internal(err)
	}
	return out, nil
}

// ApplicationAccess verifies that an account may inspect an application's
// developer-facing telemetry without leaking whether another tenant owns it.
func (s *Service) ApplicationAccess(ctx context.Context, accountID, orgID, appID string) error {
	_, err := s.memberApplication(ctx, accountID, orgID, appID)
	return err
}

type CreateKeyInput struct {
	Name           string
	Class          identity.KeyClass
	Environment    identity.Environment
	Scopes         []identity.Scope
	AllowedOrigins []string
	AllowedIPs     []string
	ExpiresAt      *time.Time
}

type CreatedKey struct {
	Key    identity.APIKey
	Secret string
}

func (s *Service) CreateKey(ctx context.Context, accountID, orgID, appID string, in CreateKeyInput, requestID, ip string) (CreatedKey, error) {
	if _, err := s.requireApplicationRole(ctx, accountID, orgID, appID, identity.OrganizationOwner, identity.OrganizationAdmin, identity.OrganizationMemberRole); err != nil {
		return CreatedKey{}, err
	}
	generated, err := identity.Generate(in.Environment)
	if err != nil {
		return CreatedKey{}, internal(err)
	}
	id, err := identity.NewID("key")
	if err != nil {
		return CreatedKey{}, internal(err)
	}
	key := identity.APIKey{
		ID: id, OrganizationID: orgID, ApplicationID: appID, Name: strings.TrimSpace(in.Name),
		Class: in.Class, Environment: in.Environment, Prefix: generated.Prefix,
		SecretHash: generated.SecretHash, Scopes: in.Scopes,
		AllowedOrigins: cleanStrings(in.AllowedOrigins), AllowedIPs: cleanStrings(in.AllowedIPs),
		ExpiresAt: in.ExpiresAt, CreatedAt: s.now().UTC(),
	}
	if err := key.Validate(); err != nil {
		return CreatedKey{}, invalid(err)
	}
	if err := s.keys.Create(ctx, key); err != nil {
		return CreatedKey{}, internal(err)
	}
	s.record(ctx, accountID, requestID, ip, audit.ActionKeyCreated, "api_key", key.ID, key.Name, nil, safeKeyAudit(key))
	return CreatedKey{Key: key, Secret: generated.Full}, nil
}

func (s *Service) Keys(ctx context.Context, accountID, orgID, appID string) ([]identity.APIKey, error) {
	if _, err := s.memberApplication(ctx, accountID, orgID, appID); err != nil {
		return nil, err
	}
	out, err := s.keys.ListByApplication(ctx, appID)
	if err != nil {
		return nil, internal(err)
	}
	return out, nil
}

func (s *Service) RevokeKey(ctx context.Context, accountID, orgID, appID, keyID, requestID, ip string) error {
	if _, err := s.requireApplicationRole(ctx, accountID, orgID, appID, identity.OrganizationOwner, identity.OrganizationAdmin, identity.OrganizationMemberRole); err != nil {
		return err
	}
	key, err := s.keys.ByIDForApplication(ctx, keyID, appID)
	if err != nil {
		return mapNotFound(err)
	}
	if key.OrganizationID != orgID {
		return notFound()
	}
	if key.RevokedAt != nil {
		return nil
	}
	now := s.now().UTC()
	if err := s.keys.Revoke(ctx, key.Prefix, now); err != nil {
		return mapNotFound(err)
	}
	s.record(ctx, accountID, requestID, ip, audit.ActionKeyRevoked, "api_key", key.ID, key.Name, safeKeyAudit(*key), map[string]any{"revokedAt": now})
	return nil
}

func (s *Service) RotateKey(ctx context.Context, accountID, orgID, appID, keyID, requestID, ip string) (CreatedKey, error) {
	if _, err := s.requireApplicationRole(ctx, accountID, orgID, appID, identity.OrganizationOwner, identity.OrganizationAdmin, identity.OrganizationMemberRole); err != nil {
		return CreatedKey{}, err
	}
	old, err := s.keys.ByIDForApplication(ctx, keyID, appID)
	if err != nil {
		return CreatedKey{}, mapNotFound(err)
	}
	if old.OrganizationID != orgID || old.RevokedAt != nil {
		return CreatedKey{}, notFound()
	}
	created, err := s.CreateKey(ctx, accountID, orgID, appID, CreateKeyInput{
		Name: old.Name, Class: old.Class, Environment: old.Environment, Scopes: old.Scopes,
		AllowedOrigins: old.AllowedOrigins, AllowedIPs: old.AllowedIPs, ExpiresAt: old.ExpiresAt,
	}, requestID, ip)
	if err != nil {
		return CreatedKey{}, err
	}
	if err := s.keys.Revoke(ctx, old.Prefix, s.now().UTC()); err != nil {
		_ = s.keys.Revoke(ctx, created.Key.Prefix, s.now().UTC())
		return CreatedKey{}, internal(fmt.Errorf("revoke rotated key: %w", err))
	}
	s.record(ctx, accountID, requestID, ip, audit.ActionKeyRotated, "api_key", old.ID, old.Name, safeKeyAudit(*old), map[string]any{"replacementId": created.Key.ID, "replacementPrefix": created.Key.Prefix})
	return created, nil
}

type CreatedInvitation struct {
	Invitation identity.OrganizationInvitation
	Token      string
}

func (s *Service) InviteMember(ctx context.Context, accountID, orgID, email string, role identity.OrganizationRole, requestID, ip string) (CreatedInvitation, error) {
	org, err := s.requireRole(ctx, accountID, orgID, identity.OrganizationOwner, identity.OrganizationAdmin)
	if err != nil {
		return CreatedInvitation{}, err
	}
	if role != identity.OrganizationAdmin && role != identity.OrganizationMemberRole && role != identity.OrganizationViewer {
		return CreatedInvitation{}, invalid(errors.New("invitation role must be ADMIN, MEMBER or VIEWER"))
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return CreatedInvitation{}, invalid(errors.New("a valid invitation email is required"))
	}
	for _, m := range org.Members {
		if strings.EqualFold(m.Email, email) {
			return CreatedInvitation{}, invalid(errors.New("that account is already a member"))
		}
	}
	token, hash, err := identity.GenerateInvitationToken()
	if err != nil {
		return CreatedInvitation{}, internal(err)
	}
	id, err := identity.NewID("invite")
	if err != nil {
		return CreatedInvitation{}, internal(err)
	}
	now := s.now().UTC()
	invite := identity.OrganizationInvitation{ID: id, OrganizationID: orgID, Email: email, Role: role, TokenHash: hash, Status: identity.InvitationPending, InvitedBy: accountID, CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)}
	if err := s.invites.Create(ctx, invite); err != nil {
		return CreatedInvitation{}, internal(err)
	}
	s.record(ctx, accountID, requestID, ip, audit.ActionOrganizationInvited, "organization", orgID, org.Name, nil, map[string]any{"invitationId": id, "email": email, "role": role, "expiresAt": invite.ExpiresAt})
	return CreatedInvitation{Invitation: invite, Token: token}, nil
}

func (s *Service) Invitations(ctx context.Context, accountID, orgID string) ([]identity.OrganizationInvitation, error) {
	if _, err := s.requireRole(ctx, accountID, orgID, identity.OrganizationOwner, identity.OrganizationAdmin); err != nil {
		return nil, err
	}
	out, err := s.invites.ListPending(ctx, orgID)
	if err != nil {
		return nil, internal(err)
	}
	return out, nil
}

func (s *Service) RevokeInvitation(ctx context.Context, accountID, orgID, inviteID string) error {
	if _, err := s.requireRole(ctx, accountID, orgID, identity.OrganizationOwner, identity.OrganizationAdmin); err != nil {
		return err
	}
	if err := s.invites.Revoke(ctx, inviteID, orgID); err != nil {
		return mapNotFound(err)
	}
	return nil
}

func (s *Service) AcceptInvitation(ctx context.Context, accountID, email, token, requestID, ip string) (identity.Organization, error) {
	invite, err := s.invites.ByTokenHash(ctx, identity.InvitationTokenHash(strings.TrimSpace(token)))
	if err != nil {
		return identity.Organization{}, mapNotFound(err)
	}
	now := s.now().UTC()
	if now.After(invite.ExpiresAt) {
		return identity.Organization{}, apierr.New(apierr.ResourceGone, "This invitation has expired.")
	}
	if !strings.EqualFold(strings.TrimSpace(email), invite.Email) {
		return identity.Organization{}, apierr.New(apierr.PermissionDenied, "This invitation belongs to another email address.")
	}
	member := identity.OrganizationMember{AccountID: accountID, Email: strings.ToLower(strings.TrimSpace(email)), Role: invite.Role, JoinedAt: now}
	if err := s.orgs.AddMember(ctx, invite.OrganizationID, member); err != nil {
		return identity.Organization{}, internal(err)
	}
	if err := s.invites.Accept(ctx, invite.ID, now); err != nil {
		return identity.Organization{}, internal(err)
	}
	org, err := s.orgs.ByIDForAccount(ctx, invite.OrganizationID, accountID)
	if err != nil {
		return identity.Organization{}, internal(err)
	}
	s.record(ctx, accountID, requestID, ip, audit.ActionOrganizationJoined, "organization", org.ID, org.Name, nil, map[string]any{"role": member.Role})
	return *org, nil
}

func (s *Service) TransferOwnership(ctx context.Context, accountID, orgID, nextOwnerID, requestID, ip string) error {
	org, err := s.requireRole(ctx, accountID, orgID, identity.OrganizationOwner)
	if err != nil {
		return err
	}
	if nextOwnerID == accountID {
		return invalid(errors.New("the selected member already owns this organization"))
	}
	found := false
	for _, m := range org.Members {
		if m.AccountID == nextOwnerID {
			found = true
			break
		}
	}
	if !found {
		return notFound()
	}
	if err := s.orgs.TransferOwnership(ctx, orgID, accountID, nextOwnerID, s.now().UTC()); err != nil {
		return mapNotFound(err)
	}
	s.record(ctx, accountID, requestID, ip, audit.ActionOrganizationOwnershipTransferred, "organization", orgID, org.Name, map[string]any{"ownerId": accountID}, map[string]any{"ownerId": nextOwnerID})
	return nil
}

func (s *Service) memberOrg(ctx context.Context, accountID, orgID string) (*identity.Organization, error) {
	org, err := s.orgs.ByIDForAccount(ctx, orgID, accountID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return org, nil
}

func (s *Service) requireRole(ctx context.Context, accountID, orgID string, allowed ...identity.OrganizationRole) (*identity.Organization, error) {
	org, err := s.memberOrg(ctx, accountID, orgID)
	if err != nil {
		return nil, err
	}
	for _, m := range org.Members {
		if m.AccountID == accountID {
			for _, role := range allowed {
				if m.Role == role {
					return org, nil
				}
			}
		}
	}
	return nil, apierr.New(apierr.PermissionDenied, "Your organization role cannot perform this action.")
}

func (s *Service) memberApplication(ctx context.Context, accountID, orgID, appID string) (*identity.Application, error) {
	if _, err := s.memberOrg(ctx, accountID, orgID); err != nil {
		return nil, err
	}
	app, err := s.apps.ByID(ctx, appID, orgID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return app, nil
}

func (s *Service) requireApplicationRole(ctx context.Context, accountID, orgID, appID string, roles ...identity.OrganizationRole) (*identity.Application, error) {
	if _, err := s.requireRole(ctx, accountID, orgID, roles...); err != nil {
		return nil, err
	}
	app, err := s.apps.ByID(ctx, appID, orgID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return app, nil
}

func (s *Service) record(ctx context.Context, actorID, requestID, ip string, action audit.Action, kind, id, label string, before, after map[string]any) {
	if s.audit == nil {
		return
	}
	e, err := audit.New(audit.Actor{Kind: audit.ActorDeveloper, ID: actorID, IP: ip}, action, audit.Target{Kind: kind, ID: id, Label: label})
	if err != nil {
		return
	}
	e.RequestID = requestID
	e = e.WithChange(before, after)
	_, _ = s.audit.Append(ctx, e)
}

func safeKeyAudit(k identity.APIKey) map[string]any {
	return map[string]any{
		"prefix": k.Prefix, "applicationId": k.ApplicationID, "organizationId": k.OrganizationID,
		"class": k.Class, "environment": k.Environment, "scopes": k.Scopes,
		"allowedOrigins": k.AllowedOrigins, "allowedIps": k.AllowedIPs, "expiresAt": k.ExpiresAt,
	}
}
func cleanStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
func invalid(err error) error { return apierr.Wrap(apierr.InvalidArgument, err.Error(), err) }
func internal(err error) error {
	return apierr.Wrap(apierr.Internal, "The developer resource could not be saved.", err)
}
func notFound() error { return apierr.New(apierr.NotFound, "The developer resource was not found.") }
func mapNotFound(err error) error {
	if errors.Is(err, identity.ErrNotFound) {
		return notFound()
	}
	return internal(err)
}
