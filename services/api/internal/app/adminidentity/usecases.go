// Package adminidentity implements global, support-only developer account
// operations without weakening the tenant-scoped developer service.
package adminidentity

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/usage"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

type Repository interface {
	Organizations(context.Context, identity.AdminListFilter) (identity.AdminPage[identity.Organization], error)
	Accounts(context.Context, identity.AdminListFilter) (identity.AdminPage[identity.DeveloperAccount], error)
	Applications(context.Context, identity.AdminListFilter) (identity.AdminPage[identity.Application], error)
	Keys(context.Context, identity.AdminListFilter) (identity.AdminPage[identity.AdminKey], error)
	UsageSummary(context.Context, string, string, time.Time) (usage.Summary, error)
	RequestLog(context.Context, string, string, *time.Time, int) (usage.Page, error)
	SetKeyState(context.Context, string, KeyMutation, audit.Entry) error
}

type KeyMutation struct {
	State  string
	At     time.Time
	Reason string
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service { return &Service{repo: repo, now: time.Now} }

func authorize(actor account.Account, permission account.Permission) error {
	if actor.Disabled || !actor.Role.Valid() || !account.Can(actor.Role, permission) {
		return apierr.New(apierr.PermissionDenied, "Your role cannot manage developer resources.")
	}
	return nil
}

func (s *Service) Organizations(ctx context.Context, actor account.Account, f identity.AdminListFilter) (identity.AdminPage[identity.Organization], error) {
	if err := authorize(actor, account.PermViewOrganization); err != nil {
		return identity.AdminPage[identity.Organization]{}, err
	}
	out, err := s.repo.Organizations(ctx, f.Normalize())
	return out, mapReadError(err)
}
func (s *Service) Accounts(ctx context.Context, actor account.Account, f identity.AdminListFilter) (identity.AdminPage[identity.DeveloperAccount], error) {
	if err := authorize(actor, account.PermViewOrganization); err != nil {
		return identity.AdminPage[identity.DeveloperAccount]{}, err
	}
	out, err := s.repo.Accounts(ctx, f.Normalize())
	return out, mapReadError(err)
}
func (s *Service) Applications(ctx context.Context, actor account.Account, f identity.AdminListFilter) (identity.AdminPage[identity.Application], error) {
	if err := authorize(actor, account.PermViewOrganization); err != nil {
		return identity.AdminPage[identity.Application]{}, err
	}
	out, err := s.repo.Applications(ctx, f.Normalize())
	return out, mapReadError(err)
}
func (s *Service) Keys(ctx context.Context, actor account.Account, f identity.AdminListFilter) (identity.AdminPage[identity.AdminKey], error) {
	if err := authorize(actor, account.PermViewOrganization); err != nil {
		return identity.AdminPage[identity.AdminKey]{}, err
	}
	f = f.Normalize()
	if f.State != "" && f.State != "active" && f.State != "suspended" && f.State != "revoked" {
		return identity.AdminPage[identity.AdminKey]{}, apierr.New(apierr.InvalidArgument, "Key state must be active, suspended or revoked.")
	}
	out, err := s.repo.Keys(ctx, f)
	return out, mapReadError(err)
}
func (s *Service) Usage(ctx context.Context, actor account.Account, orgID, appID string, since time.Time) (usage.Summary, error) {
	if err := authorize(actor, account.PermViewOrganization); err != nil {
		return usage.Summary{}, err
	}
	if strings.TrimSpace(orgID) == "" || strings.TrimSpace(appID) == "" {
		return usage.Summary{}, apierr.New(apierr.InvalidArgument, "Organization and application are required.")
	}
	out, err := s.repo.UsageSummary(ctx, orgID, appID, since)
	return out, mapReadError(err)
}
func (s *Service) Requests(ctx context.Context, actor account.Account, orgID, appID string, before *time.Time, limit int) (usage.Page, error) {
	if err := authorize(actor, account.PermViewOrganization); err != nil {
		return usage.Page{}, err
	}
	if strings.TrimSpace(orgID) == "" || strings.TrimSpace(appID) == "" {
		return usage.Page{}, apierr.New(apierr.InvalidArgument, "Organization and application are required.")
	}
	out, err := s.repo.RequestLog(ctx, orgID, appID, before, limit)
	return out, mapReadError(err)
}

func mapReadError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, identity.ErrInvalidCursor) {
		return apierr.New(apierr.InvalidArgument, "The page cursor is not valid.")
	}
	return apierr.Wrap(apierr.Internal, "Developer resources could not be loaded.", err)
}

func (s *Service) SuspendKey(ctx context.Context, actor account.Account, keyID, reason, requestID, ip string) error {
	return s.mutateKey(ctx, actor, keyID, reason, requestID, ip, "suspended", audit.ActionKeySuspended)
}
func (s *Service) RevokeKey(ctx context.Context, actor account.Account, keyID, reason, requestID, ip string) error {
	return s.mutateKey(ctx, actor, keyID, reason, requestID, ip, "revoked", audit.ActionKeyRevoked)
}
func (s *Service) mutateKey(ctx context.Context, actor account.Account, keyID, reason, requestID, ip, state string, action audit.Action) error {
	if err := authorize(actor, account.PermSuspendKey); err != nil {
		return err
	}
	keyID, reason = strings.TrimSpace(keyID), strings.TrimSpace(reason)
	if keyID == "" || reason == "" {
		return apierr.New(apierr.InvalidArgument, "Key and reason are required.")
	}
	e, err := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: actor.ID, Label: actor.Email, IP: ip}, action, audit.Target{Kind: "api_key", ID: keyID})
	if err != nil {
		return apierr.Wrap(apierr.Internal, "Could not prepare the audit entry.", err)
	}
	e.RequestID, e.Reason = requestID, reason
	e = e.WithChange(nil, map[string]any{"state": state, "reason": reason})
	if err := s.repo.SetKeyState(ctx, keyID, KeyMutation{State: state, At: s.now().UTC(), Reason: reason}, e); err != nil {
		if errors.Is(err, identity.ErrNotFound) {
			return apierr.New(apierr.NotFound, "The API key was not found.")
		}
		if errors.Is(err, identity.ErrInvalidKeyTransition) {
			return apierr.New(apierr.Conflict, "The API key cannot move to that state.")
		}
		return apierr.Wrap(apierr.Internal, "The API key state could not be changed.", err)
	}
	return nil
}
