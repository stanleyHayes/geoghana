package account

import (
	"context"
	"errors"
	"strings"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
)

// ErrCannotActOnSelf refuses an operator's attempt to change their own role or
// disable themselves.
//
// Both are foot-guns rather than attacks: demoting yourself out of role:manage,
// or disabling the only account that holds it, leaves the workspace with no way
// back in except editing the database by hand.
var ErrCannotActOnSelf = errors.New("account: an operator cannot change their own role or access")

// ErrUnknownStaffTarget is returned for a missing or unusable target account.
var ErrUnknownStaffTarget = errors.New("account: no such account")

// ErrInvalidStaffRole is returned when the requested role is not one this system
// recognises.
var ErrInvalidStaffRole = errors.New("account: unknown role")

// StaffActor is who is performing the change, for the audit record.
type StaffActor struct {
	ID        string
	Email     string
	IP        string
	RequestID string
}

// SetStaffRole moves an operator to a different role.
//
// Sessions are revoked afterwards: a role lives in the session the caller is
// already holding, so without this a demotion would not take effect until that
// session happened to expire.
func (s *Service) SetStaffRole(ctx context.Context, actor StaffActor, targetID string, role account.Role) error {
	if strings.TrimSpace(targetID) == "" {
		return ErrUnknownStaffTarget
	}
	if targetID == actor.ID {
		return ErrCannotActOnSelf
	}
	if !role.Valid() {
		return ErrInvalidStaffRole
	}
	target, err := s.accounts.ByID(ctx, targetID)
	if err != nil {
		return err
	}
	previous := target.Role
	if previous == role {
		return nil
	}
	if err := s.accounts.SetRole(ctx, targetID, role); err != nil {
		return err
	}
	if _, err := s.sessions.RevokeAllForAccount(ctx, targetID); err != nil {
		s.log.WarnContext(ctx, "role changed but sessions not revoked", "account", targetID, "err", err)
	}
	s.recordStaffChange(ctx, actor, "account.role_changed", target.Email, targetID,
		map[string]any{"role": string(previous)}, map[string]any{"role": string(role)})
	return nil
}

// SetStaffDisabled locks an operator out, or lets them back in.
//
// Disabling is the containment lever: every sign-in and every privileged action
// already checks the flag, but nothing in shipped code could set it, so a
// compromised operator could only be dealt with by revoking their API keys one
// at a time. Live sessions are revoked too, otherwise disabling would not stop
// somebody already signed in.
func (s *Service) SetStaffDisabled(ctx context.Context, actor StaffActor, targetID string, disabled bool) error {
	if strings.TrimSpace(targetID) == "" {
		return ErrUnknownStaffTarget
	}
	if targetID == actor.ID {
		return ErrCannotActOnSelf
	}
	target, err := s.accounts.ByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target.Disabled == disabled {
		return nil
	}
	if err := s.accounts.SetDisabled(ctx, targetID, disabled); err != nil {
		return err
	}
	if disabled {
		if _, err := s.sessions.RevokeAllForAccount(ctx, targetID); err != nil {
			s.log.WarnContext(ctx, "account disabled but sessions not revoked", "account", targetID, "err", err)
		}
	}
	action := "account.reinstated"
	if disabled {
		action = "account.disabled"
	}
	s.recordStaffChange(ctx, actor, action, target.Email, targetID,
		map[string]any{"disabled": !disabled}, map[string]any{"disabled": disabled})
	return nil
}

// recordStaffChange writes the change to the hash-chained trail.
//
// A failure to record is logged rather than returned: the change has already
// happened, and reporting it as failed would be a worse lie than a gap in the
// trail that the chain itself will expose.
func (s *Service) recordStaffChange(
	ctx context.Context,
	actor StaffActor,
	action string,
	targetEmail string,
	targetID string,
	before map[string]any,
	after map[string]any,
) {
	if s.audit == nil {
		return
	}
	e, err := audit.New(
		audit.Actor{Kind: audit.ActorAdmin, ID: actor.ID, Label: actor.Email, IP: actor.IP},
		audit.Action(action),
		audit.Target{Kind: "account", ID: targetID, Label: targetEmail},
	)
	if err != nil {
		s.log.WarnContext(ctx, "staff change not recorded", "action", action, "err", err)
		return
	}
	e = e.WithRequest(actor.RequestID).WithChange(before, after)
	if _, err := s.audit.Append(ctx, e); err != nil {
		s.log.WarnContext(ctx, "staff change not recorded", "action", action, "err", err)
	}
}
