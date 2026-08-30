package fairuse

import (
	"context"
	"strings"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

type Actor struct {
	ID, Email, IP, RequestID string
	Role                     account.Role
}

type Repository interface {
	CurrentPolicy(context.Context, time.Time) (*identity.FairUsePolicy, error)
	CurrentOverride(context.Context, string, time.Time) (*identity.FairUseOverride, error)
	AppendPolicyAudited(context.Context, identity.FairUsePolicy, audit.Entry) error
	AppendOverrideAudited(context.Context, identity.FairUseOverride, audit.Entry) error
}

type AuditSink interface {
	Append(context.Context, audit.Entry) (audit.Entry, error)
}
type Invalidator interface{ Invalidate() }

type Service struct {
	repo        Repository
	audit       AuditSink
	invalidator Invalidator
	now         func() time.Time
}

func NewService(repo Repository, sink AuditSink, invalidator Invalidator) *Service {
	return &Service{repo: repo, audit: sink, invalidator: invalidator, now: time.Now}
}

func (s *Service) CurrentPolicy(ctx context.Context) (identity.FairUsePolicy, error) {
	p, err := s.repo.CurrentPolicy(ctx, s.now().UTC())
	if err != nil {
		return identity.FairUsePolicy{}, apierr.Wrap(apierr.Internal, "Could not read the fair-use policy.", err)
	}
	if p == nil {
		return identity.DefaultFairUsePolicy(), nil
	}
	return *p, nil
}

func (s *Service) AppendPolicy(ctx context.Context, actor Actor, p identity.FairUsePolicy) (identity.FairUsePolicy, error) {
	p.ID = identity.DefaultFairUsePolicyID
	p.ActorID = actor.ID
	p.CreatedAt = s.now().UTC()
	validation := p.Validate()
	if validation == nil {
		current, err := s.CurrentPolicy(ctx)
		if err != nil {
			validation = err
		} else if p.Revision <= current.Revision {
			validation = apierr.New(apierr.InvalidArgument, "Policy revision must increase.")
		}
	}
	err := s.mutate(ctx, actor, audit.ActionFairUsePolicyAppended, "fair_use_policy", p.ID, p.Reason, policyState(p), func(e audit.Entry) error { return s.repo.AppendPolicyAudited(ctx, p, e) }, validation)
	return p, err
}

func (s *Service) AppendOverride(ctx context.Context, actor Actor, o identity.FairUseOverride) (identity.FairUseOverride, error) {
	o.ActorID = actor.ID
	o.CreatedAt = s.now().UTC()
	if o.ID == "" {
		o.ID, _ = identity.NewID("fuo")
	}
	validation := o.Validate()
	if validation == nil && o.Enabled {
		p, err := s.CurrentPolicy(ctx)
		if err != nil {
			validation = err
		} else if !o.Raises(p.Authenticated, p.CostCeilings) {
			validation = identity.ErrInvalidFairUseOverride
		}
	}
	err := s.mutate(ctx, actor, audit.ActionFairUseOverrideAppended, "fair_use_override", o.ID, o.Reason, overrideState(o), func(e audit.Entry) error { return s.repo.AppendOverrideAudited(ctx, o, e) }, validation)
	return o, err
}

func (s *Service) mutate(ctx context.Context, actor Actor, action audit.Action, kind, id, reason string, after map[string]any, persist func(audit.Entry) error, validation error) error {
	var permissionErr error
	if !account.Can(actor.Role, account.PermManageFairUse) {
		permissionErr = apierr.New(apierr.PermissionDenied, "Your role cannot manage fair-use policy.").WithDetail("requiredPermission", string(account.PermManageFairUse))
		validation = permissionErr
	}
	if strings.TrimSpace(reason) == "" && validation == nil {
		validation = apierr.New(apierr.InvalidArgument, "A documented reason is required.")
	}
	e, err := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: actor.ID, Label: actor.Email, IP: actor.IP}, action, audit.Target{Kind: kind, ID: id})
	if err != nil {
		return apierr.Wrap(apierr.Internal, "Could not prepare audit evidence.", err)
	}
	e = e.WithRequest(actor.RequestID).WithReason(reason).WithChange(nil, after)
	if validation != nil {
		e = e.Failed(validation)
		if s.audit != nil {
			_, _ = s.audit.Append(ctx, e)
		}
		if permissionErr != nil {
			return permissionErr
		}
		return apierr.Wrap(apierr.InvalidArgument, validation.Error(), validation)
	}
	if err := persist(e); err != nil {
		if s.audit != nil {
			_, _ = s.audit.Append(ctx, e.Failed(err))
		}
		return apierr.Wrap(apierr.Internal, "Could not append fair-use policy.", err)
	}
	if s.invalidator != nil {
		s.invalidator.Invalidate()
	}
	return nil
}

func allowanceState(a identity.Allowance) map[string]any {
	return map[string]any{"burstUnits": a.BurstUnits, "refillPerSecond": a.RefillPerSecond, "windowSeconds": int64(a.Window.Seconds())}
}
func ceilingsState(c map[identity.CostClass]int) map[string]any {
	out := map[string]any{}
	for k, v := range c {
		out[string(k)] = v
	}
	return out
}
func policyState(p identity.FairUsePolicy) map[string]any {
	return map[string]any{"revision": p.Revision, "anonymous": allowanceState(p.Anonymous), "authenticated": allowanceState(p.Authenticated), "sandbox": allowanceState(p.Sandbox), "costCeilings": ceilingsState(p.CostCeilings), "effectiveFrom": p.EffectiveFrom}
}
func overrideState(o identity.FairUseOverride) map[string]any {
	return map[string]any{"applicationId": o.ApplicationID, "allowance": allowanceState(o.Allowance), "costCeilings": ceilingsState(o.CostCeilings), "enabled": o.Enabled, "expiresAt": o.ExpiresAt, "supersedesId": o.SupersedesID}
}
