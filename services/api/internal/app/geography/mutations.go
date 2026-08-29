package geography

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

// Geography mutations (story GEO-17.3).
//
// Three rules hold for every one of them, and they are enforced here rather
// than in a handler so all transports inherit them:
//
//  1. The caller's PERMISSION is checked, not their role name. UI gating is
//     presentation only.
//  2. Every mutation writes an audit row, including the ones that fail.
//  3. Deprecation creates a redirect and never deletes. A hard delete breaks
//     every stored identifier pointing at the record (GEO-3.3).

// Actor is who is performing a mutation.
type Actor struct {
	ID    string
	Email string
	Role  account.Role
	IP    string
	// RequestID ties the audit row to the access log for the same request.
	RequestID string
}

// AuditSink records privileged actions. An interface so the use cases can be
// tested without a database.
type AuditSink interface {
	Append(ctx context.Context, e audit.Entry) (audit.Entry, error)
}

// WithMutations enables writing. The redirect repository is already held for
// reads (resolving a merged id), and deprecation writes through the same one —
// a second path to the same collection would be a place for them to disagree.
func (s *Service) WithMutations(sink AuditSink) *Service {
	s.auditSink = sink
	return s
}

func (s *Service) authorize(a Actor, p account.Permission) error {
	if !account.Can(a.Role, p) {
		return apierr.New(apierr.PermissionDenied, "Your role cannot perform this action.").
			WithDetail("requiredPermission", string(p))
	}
	return nil
}

// record writes an audit row. A failure to record is logged by the sink, never
// fatal: an audit gap is bad, refusing a legitimate steward action because of
// one is worse — and the alternative is a steward who cannot work.
func (s *Service) record(
	ctx context.Context, a Actor, action audit.Action, target audit.Target,
	before, after map[string]any, cause error,
) {
	if s.auditSink == nil {
		return
	}
	e, err := audit.New(
		audit.Actor{Kind: audit.ActorAdmin, ID: a.ID, Label: a.Email, IP: a.IP},
		action, target,
	)
	if err != nil {
		return
	}
	e = e.WithChange(before, after).WithRequest(a.RequestID)
	if cause != nil {
		e = e.Failed(cause)
	}
	_, _ = s.auditSink.Append(ctx, e)
}

func regionState(r domain.Region) map[string]any {
	return map[string]any{
		"name": r.Name, "capital": r.Capital, "code": r.OfficialCode,
		"status": string(r.Status), "verificationStatus": string(r.VerificationStatus),
	}
}

func districtState(d domain.District) map[string]any {
	return map[string]any{
		"name": d.Name, "regionId": d.RegionID, "type": d.DistrictType,
		"code": d.OfficialCode, "capital": d.Capital,
		"status": string(d.Status), "verificationStatus": string(d.VerificationStatus),
	}
}

func placeState(p domain.Place) map[string]any {
	return map[string]any{
		"name": p.Name, "type": string(p.Type), "regionId": p.RegionID,
		"districtId": p.DistrictID, "status": string(p.Status),
		"verificationStatus": string(p.VerificationStatus),
	}
}

// UpdateRegion edits a region's editable fields.
//
// Identity fields — the id and the country — are deliberately not editable.
// Changing an id silently breaks every consumer holding it, which is what
// redirects exist to prevent.
func (s *Service) UpdateRegion(
	ctx context.Context, a Actor, id string, changes RegionChanges,
) (*domain.Region, error) {
	if err := s.authorize(a, account.PermEditGeography); err != nil {
		s.record(ctx, a, audit.ActionRecordUpdated,
			audit.Target{Kind: "region", ID: id}, nil, nil, err)
		return nil, err
	}
	current, err := s.GetRegion(ctx, id)
	if err != nil {
		return nil, err
	}
	before := regionState(*current)

	next := *current
	changes.applyTo(&next)
	if err := next.Validate(); err != nil {
		werr := apierr.Wrap(apierr.InvalidArgument, err.Error(), err)
		s.record(ctx, a, audit.ActionRecordUpdated,
			audit.Target{Kind: "region", ID: id, Label: current.Name}, before, nil, werr)
		return nil, werr
	}

	_, err = s.regions.Upsert(ctx, next)
	s.record(ctx, a, audit.ActionRecordUpdated,
		audit.Target{Kind: "region", ID: id, Label: next.Name},
		before, regionState(next), err)
	if err != nil {
		return nil, apierr.Wrap(apierr.Internal, "Could not save the region.", err)
	}
	return &next, nil
}

// RegionChanges carries only the fields a steward may edit. Pointers so an
// omitted field is distinguishable from one deliberately cleared.
type RegionChanges struct {
	Name               *string
	Capital            *string
	OfficialCode       *string
	VerificationStatus *string
}

func (c RegionChanges) applyTo(r *domain.Region) {
	if c.Name != nil {
		r.Name = *c.Name
	}
	if c.Capital != nil {
		r.Capital = *c.Capital
	}
	if c.OfficialCode != nil {
		r.OfficialCode = *c.OfficialCode
	}
	if c.VerificationStatus != nil {
		r.VerificationStatus = domain.VerificationStatus(*c.VerificationStatus)
	}
}

// DistrictChanges carries the editable fields of a district.
type DistrictChanges struct {
	Name               *string
	DistrictType       *string
	OfficialCode       *string
	Capital            *string
	RegionID           *string
	VerificationStatus *string
}

func (c DistrictChanges) applyTo(d *domain.District) {
	if c.Name != nil {
		d.Name = *c.Name
	}
	if c.DistrictType != nil {
		d.DistrictType = *c.DistrictType
	}
	if c.OfficialCode != nil {
		d.OfficialCode = *c.OfficialCode
	}
	if c.Capital != nil {
		d.Capital = *c.Capital
	}
	if c.RegionID != nil {
		d.RegionID = *c.RegionID
	}
	if c.VerificationStatus != nil {
		d.VerificationStatus = domain.VerificationStatus(*c.VerificationStatus)
	}
}

func (s *Service) UpdateDistrict(
	ctx context.Context, a Actor, id string, changes DistrictChanges,
) (*domain.District, error) {
	if err := s.authorize(a, account.PermEditGeography); err != nil {
		s.record(ctx, a, audit.ActionRecordUpdated,
			audit.Target{Kind: "district", ID: id}, nil, nil, err)
		return nil, err
	}
	current, err := s.GetDistrict(ctx, id)
	if err != nil {
		return nil, err
	}
	before := districtState(*current)

	next := *current
	changes.applyTo(&next)
	if err := next.Validate(); err != nil {
		werr := apierr.Wrap(apierr.InvalidArgument, err.Error(), err)
		s.record(ctx, a, audit.ActionRecordUpdated,
			audit.Target{Kind: "district", ID: id, Label: current.Name}, before, nil, werr)
		return nil, werr
	}

	_, err = s.districts.Upsert(ctx, next)
	s.record(ctx, a, audit.ActionRecordUpdated,
		audit.Target{Kind: "district", ID: id, Label: next.Name},
		before, districtState(next), err)
	if err != nil {
		return nil, apierr.Wrap(apierr.Internal, "Could not save the district.", err)
	}
	return &next, nil
}

// PlaceChanges carries the editable fields of a place.
type PlaceChanges struct {
	Name               *string
	Type               *string
	DistrictID         *string
	RegionID           *string
	VerificationStatus *string
}

func (c PlaceChanges) applyTo(p *domain.Place) {
	if c.Name != nil {
		p.Name = *c.Name
	}
	if c.Type != nil {
		p.Type = domain.PlaceType(*c.Type)
	}
	if c.DistrictID != nil {
		p.DistrictID = *c.DistrictID
	}
	if c.RegionID != nil {
		p.RegionID = *c.RegionID
	}
	if c.VerificationStatus != nil {
		p.VerificationStatus = domain.VerificationStatus(*c.VerificationStatus)
	}
}

func (s *Service) UpdatePlace(
	ctx context.Context, a Actor, id string, changes PlaceChanges,
) (*domain.Place, error) {
	if err := s.authorize(a, account.PermEditGeography); err != nil {
		s.record(ctx, a, audit.ActionRecordUpdated,
			audit.Target{Kind: "place", ID: id}, nil, nil, err)
		return nil, err
	}
	current, err := s.GetPlace(ctx, id)
	if err != nil {
		return nil, err
	}
	before := placeState(*current)

	next := *current
	changes.applyTo(&next)
	if err := next.Validate(); err != nil {
		werr := apierr.Wrap(apierr.InvalidArgument, err.Error(), err)
		s.record(ctx, a, audit.ActionRecordUpdated,
			audit.Target{Kind: "place", ID: id, Label: current.Name}, before, nil, werr)
		return nil, werr
	}

	_, err = s.places.Upsert(ctx, next)
	s.record(ctx, a, audit.ActionRecordUpdated,
		audit.Target{Kind: "place", ID: id, Label: next.Name},
		before, placeState(next), err)
	if err != nil {
		return nil, apierr.Wrap(apierr.Internal, "Could not save the place.", err)
	}
	return &next, nil
}

// ErrCannotMergeIntoSelf guards the obvious mistake.
var ErrCannotMergeIntoSelf = errors.New("a record cannot be merged into itself")

// DeprecatePlace retires a place, optionally redirecting to a survivor.
//
// It NEVER deletes. Every consumer holding the old id keeps working: a lookup
// returns 410 with `mergedInto` so they can follow it and update their store
// at their own pace. Hard-deleting would turn every stored reference into a
// 404 with no way to recover the correct successor (GEO-3.3, Spec §18).
func (s *Service) DeprecatePlace(
	ctx context.Context, a Actor, id, mergedInto, reason string,
) error {
	if err := s.authorize(a, account.PermEditGeography); err != nil {
		s.record(ctx, a, audit.ActionRecordDeprecated,
			audit.Target{Kind: "place", ID: id}, nil, nil, err)
		return err
	}
	if mergedInto == id {
		// The message is written for the reader, not copied from the sentinel:
		// Wrap appends the cause, so passing err.Error() as the message prints
		// it twice.
		err := apierr.Wrap(apierr.InvalidArgument,
			"A record cannot be merged into itself.", ErrCannotMergeIntoSelf)
		s.record(ctx, a, audit.ActionRecordDeprecated,
			audit.Target{Kind: "place", ID: id}, nil, nil, err)
		return err
	}
	current, err := s.GetPlace(ctx, id)
	if err != nil {
		return err
	}
	if mergedInto != "" {
		// The survivor must exist, or the redirect points nowhere and a
		// consumer following it gets a 404 from a 410 — worse than either.
		if _, err := s.GetPlace(ctx, mergedInto); err != nil {
			werr := apierr.New(apierr.InvalidArgument, "The record to merge into does not exist.").
				WithDetail("mergedInto", mergedInto)
			s.record(ctx, a, audit.ActionRecordDeprecated,
				audit.Target{Kind: "place", ID: id, Label: current.Name}, nil, nil, werr)
			return werr
		}
	}

	before := placeState(*current)
	next := *current
	if mergedInto != "" {
		next.Status = domain.StatusMerged
	} else {
		next.Status = domain.StatusDeprecated
	}

	if _, err := s.places.Upsert(ctx, next); err != nil {
		s.record(ctx, a, audit.ActionRecordDeprecated,
			audit.Target{Kind: "place", ID: id, Label: current.Name}, before, nil, err)
		return apierr.Wrap(apierr.Internal, "Could not deprecate the place.", err)
	}

	if s.redirects != nil && mergedInto != "" {
		if err := s.redirects.Put(ctx, domain.Redirect{
			OldID: id, NewID: mergedInto, Reason: reason,
			MergedAt: time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			// The record is already deprecated; without the redirect its id
			// would 404 instead of 410. Report loudly rather than pretend.
			s.record(ctx, a, audit.ActionRecordDeprecated,
				audit.Target{Kind: "place", ID: id, Label: current.Name}, before, nil, err)
			return apierr.Wrap(apierr.Internal,
				"The place was deprecated but its redirect could not be written.", err)
		}
	}

	after := placeState(next)
	if mergedInto != "" {
		after["mergedInto"] = mergedInto
	}
	if reason != "" {
		after["reason"] = reason
	}
	s.record(ctx, a, audit.ActionRecordDeprecated,
		audit.Target{Kind: "place", ID: id, Label: current.Name}, before, after, nil)
	return nil
}

var _ = fmt.Sprintf
