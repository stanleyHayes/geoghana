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
	orgs  OrganizationRepository
	apps  ApplicationRepository
	keys  KeyRepository
	audit AuditRepository
	now   func() time.Time
}

type OrganizationRepository interface {
	Create(context.Context, identity.Organization) error
	ByIDForOwner(context.Context, string, string) (*identity.Organization, error)
	ListByOwner(context.Context, string) ([]identity.Organization, error)
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

func NewService(orgs OrganizationRepository, apps ApplicationRepository, keys KeyRepository, auditRepo AuditRepository) *Service {
	return &Service{orgs: orgs, apps: apps, keys: keys, audit: auditRepo, now: time.Now}
}

func (s *Service) CreateOrganization(ctx context.Context, accountID, name, requestID, ip string) (identity.Organization, error) {
	id, err := identity.NewID("org")
	if err != nil {
		return identity.Organization{}, internal(err)
	}
	org := identity.Organization{ID: id, Name: strings.TrimSpace(name), OwnerID: accountID, CreatedAt: s.now().UTC()}
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
	out, err := s.orgs.ListByOwner(ctx, accountID)
	if err != nil {
		return nil, internal(err)
	}
	return out, nil
}

func (s *Service) CreateApplication(ctx context.Context, accountID, orgID, name, description, requestID, ip string) (identity.Application, error) {
	if _, err := s.ownedOrg(ctx, accountID, orgID); err != nil {
		return identity.Application{}, err
	}
	id, err := identity.NewID("app")
	if err != nil {
		return identity.Application{}, internal(err)
	}
	app := identity.Application{ID: id, OrganizationID: orgID, Name: strings.TrimSpace(name), Description: strings.TrimSpace(description), CreatedAt: s.now().UTC()}
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
	if _, err := s.ownedOrg(ctx, accountID, orgID); err != nil {
		return nil, err
	}
	out, err := s.apps.ListByOrganization(ctx, orgID)
	if err != nil {
		return nil, internal(err)
	}
	return out, nil
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
	if _, err := s.ownedApplication(ctx, accountID, orgID, appID); err != nil {
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
	if _, err := s.ownedApplication(ctx, accountID, orgID, appID); err != nil {
		return nil, err
	}
	out, err := s.keys.ListByApplication(ctx, appID)
	if err != nil {
		return nil, internal(err)
	}
	return out, nil
}

func (s *Service) RevokeKey(ctx context.Context, accountID, orgID, appID, keyID, requestID, ip string) error {
	if _, err := s.ownedApplication(ctx, accountID, orgID, appID); err != nil {
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
	if _, err := s.ownedApplication(ctx, accountID, orgID, appID); err != nil {
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

func (s *Service) ownedOrg(ctx context.Context, accountID, orgID string) (*identity.Organization, error) {
	org, err := s.orgs.ByIDForOwner(ctx, orgID, accountID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return org, nil
}

func (s *Service) ownedApplication(ctx context.Context, accountID, orgID, appID string) (*identity.Application, error) {
	if _, err := s.ownedOrg(ctx, accountID, orgID); err != nil {
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
