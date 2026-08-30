package geography

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

type CreateInput struct {
	ID, Name, CountryCode, Capital, OfficialCode, RegionID, RegionName, DistrictID, DistrictName string
	DistrictType, PlaceType, RoadClass, Ref, POIClass, Category, Attribution                     string
	Centroid                                                                                     *domain.Coordinate
	Geometry                                                                                     *domain.Geometry
	Provenance                                                                                   domain.Provenance
}

func (s *Service) mutationEvidence(a Actor, action audit.Action, kind, id string, before, after map[string]any) (audit.Entry, error) {
	e, err := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: a.ID, Label: a.Email, IP: a.IP}, action, audit.Target{Kind: kind, ID: id})
	if err != nil {
		return audit.Entry{}, err
	}
	return e.WithChange(before, after).WithRequest(a.RequestID), nil
}

func (s *Service) CreateAdminGeography(ctx context.Context, a Actor, kind string, in CreateInput) (any, error) {
	if err := s.authorize(a, account.PermEditGeography); err != nil {
		return nil, err
	}
	if s.admin == nil {
		return nil, apierr.New(apierr.Internal, "Admin geography writes are not configured.")
	}
	baseStatus, verify := domain.StatusActive, domain.VerificationReviewed
	var value any
	switch kind {
	case "region":
		value = domain.Region{ID: in.ID, CountryCode: in.CountryCode, Name: in.Name, Capital: in.Capital, OfficialCode: in.OfficialCode, Status: baseStatus, VerificationStatus: verify, Centroid: in.Centroid, Geometry: in.Geometry, Provenance: in.Provenance, DatasetVersion: s.version}
	case "district":
		value = domain.District{ID: in.ID, RegionID: in.RegionID, RegionName: in.RegionName, Name: in.Name, DistrictType: in.DistrictType, OfficialCode: in.OfficialCode, Capital: in.Capital, Status: baseStatus, VerificationStatus: verify, Centroid: in.Centroid, Geometry: in.Geometry, Provenance: in.Provenance, DatasetVersion: s.version}
	case "place":
		pt, err := domain.ParsePlaceType(in.PlaceType)
		if err != nil {
			return nil, apierr.Wrap(apierr.InvalidArgument, "Unknown place type.", err)
		}
		value = domain.Place{ID: in.ID, Name: in.Name, Type: pt, RegionID: in.RegionID, RegionName: in.RegionName, DistrictID: in.DistrictID, DistrictName: in.DistrictName, Status: baseStatus, VerificationStatus: verify, Centroid: in.Centroid, Geometry: in.Geometry, Provenance: in.Provenance, DatasetVersion: s.version}
	case "road":
		rc, ok := domain.ParseRoadClass(in.RoadClass)
		if !ok {
			return nil, apierr.New(apierr.InvalidArgument, "Unknown road class.")
		}
		value = domain.Road{ID: in.ID, Name: in.Name, Ref: in.Ref, Class: rc, RegionID: in.RegionID, RegionName: in.RegionName, DistrictID: in.DistrictID, Geometry: in.Geometry, Status: baseStatus, VerificationStatus: verify, Provenance: in.Provenance, Attribution: in.Attribution, DatasetVersion: s.version}
	case "poi":
		value = domain.POI{ID: in.ID, Name: in.Name, Class: domain.POIClass(strings.ToUpper(in.POIClass)), Category: in.Category, RegionID: in.RegionID, RegionName: in.RegionName, DistrictID: in.DistrictID, Centroid: in.Centroid, Status: baseStatus, VerificationStatus: verify, Provenance: in.Provenance, Attribution: in.Attribution, DatasetVersion: s.version}
	default:
		return nil, apierr.New(apierr.InvalidArgument, "Unsupported geography kind.")
	}
	if !domain.IsULID(in.ID) {
		return nil, apierr.New(apierr.InvalidArgument, "A valid ULID id is required.")
	}
	if in.Geometry != nil {
		if err := in.Geometry.Validate(); err != nil {
			return nil, apierr.Wrap(apierr.InvalidCoordinate, "Geometry is invalid.", err)
		}
	}
	if kind == "district" && in.Geometry != nil {
		if s.regionBoundaries == nil {
			return nil, apierr.New(apierr.Internal, "Region boundaries are not configured.")
		}
		parent, err := s.regionBoundaries.GetGeometry(ctx, in.RegionID)
		if err != nil || parent == nil || !parent.ContainsGeometry(in.Geometry) {
			return nil, apierr.New(apierr.InvalidCoordinate, "District boundary must be contained by its region boundary.")
		}
	}
	var err error
	switch v := value.(type) {
	case domain.Region:
		err = v.Validate()
	case domain.District:
		err = v.Validate()
	case domain.Place:
		err = v.Validate()
	case domain.Road:
		err = v.Validate()
		if err == nil {
			err = v.Provenance.Validate()
		}
	case domain.POI:
		err = v.Validate()
		if err == nil {
			err = v.Provenance.Validate()
		}
	}
	if err != nil {
		return nil, apierr.Wrap(apierr.InvalidArgument, "Geography record is invalid.", err)
	}
	e, _ := s.mutationEvidence(a, audit.ActionRecordUpdated, kind, in.ID, nil, map[string]any{"name": in.Name, "status": "ACTIVE"})
	switch v := value.(type) {
	case domain.Region:
		err = s.admin.CreateRegion(ctx, v, e)
	case domain.District:
		err = s.admin.CreateDistrict(ctx, v, e)
	case domain.Place:
		err = s.admin.CreatePlace(ctx, v, e)
	case domain.Road:
		err = s.admin.CreateRoad(ctx, v, e)
	case domain.POI:
		err = s.admin.CreatePOI(ctx, v, e)
	}
	if err != nil {
		return nil, apierr.Wrap(apierr.Conflict, "Could not create geography record.", err)
	}
	return value, nil
}

func (s *Service) DeprecateAdminGeography(ctx context.Context, a Actor, kind, id, target, reason string) error {
	if err := s.authorize(a, account.PermEditGeography); err != nil {
		return err
	}
	if s.admin == nil {
		return apierr.New(apierr.Internal, "Admin geography writes are not configured.")
	}
	e, _ := s.mutationEvidence(a, audit.ActionRecordDeprecated, kind, id, nil, map[string]any{"status": map[bool]string{true: "MERGED", false: "DEPRECATED"}[target != ""], "mergedInto": target, "reason": reason})
	if err := s.admin.Deprecate(ctx, kind, id, target, reason, e); err != nil {
		return apierr.Wrap(apierr.Conflict, "Could not deprecate geography record.", err)
	}
	return nil
}
func (s *Service) ListAdminRedirects(ctx context.Context, a Actor, p ports.ListParams) (ports.Page[domain.Redirect], error) {
	if err := s.authorize(a, account.PermViewGeography); err != nil {
		return ports.Page[domain.Redirect]{}, err
	}
	return s.admin.ListRedirects(ctx, p)
}
func (s *Service) ListAdminAliases(ctx context.Context, a Actor, placeID string) ([]domain.Alias, error) {
	if err := s.authorize(a, account.PermViewGeography); err != nil {
		return nil, err
	}
	return s.admin.ListAliases(ctx, placeID)
}
func (s *Service) CreateAdminAlias(ctx context.Context, a Actor, x domain.Alias) error {
	if err := s.authorize(a, account.PermEditGeography); err != nil {
		return err
	}
	if !domain.IsULID(x.ID) || !domain.IsULID(x.PlaceID) || strings.TrimSpace(x.Value) == "" {
		return apierr.New(apierr.InvalidArgument, "Alias id, place id and value are required.")
	}
	x.Status = domain.StatusActive
	e, _ := s.mutationEvidence(a, audit.ActionRecordUpdated, "place_alias", x.ID, nil, map[string]any{"placeId": x.PlaceID, "value": x.Value, "status": "ACTIVE"})
	if err := s.admin.CreateAlias(ctx, x, e); err != nil {
		return apierr.Wrap(apierr.Conflict, "Could not create alias.", err)
	}
	return nil
}
func (s *Service) DeprecateAdminAlias(ctx context.Context, a Actor, placeID, id string) error {
	if err := s.authorize(a, account.PermEditGeography); err != nil {
		return err
	}
	e, _ := s.mutationEvidence(a, audit.ActionRecordDeprecated, "place_alias", id, nil, map[string]any{"placeId": placeID, "status": "DEPRECATED"})
	if err := s.admin.DeprecateAlias(ctx, placeID, id, e); err != nil {
		return apierr.Wrap(apierr.Conflict, "Could not deprecate alias.", err)
	}
	return nil
}

// BoundaryResult carries the representation and its strong validator. Clients
// must return ETag in If-Match before a write, preventing lost updates.
type BoundaryResult struct {
	Geometry *domain.Geometry
	ETag     string
}

func boundaryETag(g *domain.Geometry) string {
	b, _ := json.Marshal(g)
	s := sha256.Sum256(b)
	return `"` + hex.EncodeToString(s[:]) + `"`
}

func (s *Service) GetAdminBoundary(ctx context.Context, a Actor, kind, id string) (BoundaryResult, error) {
	if err := s.authorize(a, account.PermViewGeography); err != nil {
		return BoundaryResult{}, err
	}
	repo := s.regionBoundaries
	if kind == "district" {
		repo = s.districtBoundaries
	}
	if repo == nil || (kind != "region" && kind != "district") {
		return BoundaryResult{}, apierr.New(apierr.InvalidArgument, "Unsupported boundary kind.")
	}
	g, err := repo.GetGeometry(ctx, id)
	if err != nil {
		return BoundaryResult{}, apierr.Wrap(apierr.NotFound, "Boundary record not found.", err)
	}
	return BoundaryResult{Geometry: g, ETag: boundaryETag(g)}, nil
}

func (s *Service) UpdateAdminBoundary(ctx context.Context, a Actor, kind, id, ifMatch string, next *domain.Geometry) (BoundaryResult, error) {
	if err := s.authorize(a, account.PermEditGeography); err != nil {
		return BoundaryResult{}, err
	}
	if err := s.authorize(a, account.PermEditGeometry); err != nil {
		return BoundaryResult{}, err
	}
	if next == nil || (next.Type != domain.GeomPolygon && next.Type != domain.GeomMultiPolygon) {
		return BoundaryResult{}, apierr.New(apierr.InvalidArgument, "A Polygon or MultiPolygon geometry is required.")
	}
	if err := next.Validate(); err != nil {
		return BoundaryResult{}, apierr.Wrap(apierr.InvalidCoordinate, "Boundary geometry is invalid.", err)
	}
	repo := s.regionBoundaries
	if kind == "district" {
		repo = s.districtBoundaries
	}
	if repo == nil || (kind != "region" && kind != "district") {
		return BoundaryResult{}, apierr.New(apierr.InvalidArgument, "Unsupported boundary kind.")
	}
	current, err := repo.GetGeometry(ctx, id)
	if err != nil {
		return BoundaryResult{}, apierr.Wrap(apierr.NotFound, "Boundary record not found.", err)
	}
	if ifMatch == "" || ifMatch != boundaryETag(current) {
		return BoundaryResult{}, apierr.New(apierr.Conflict, "Boundary changed since it was loaded.").WithDetail("currentETag", boundaryETag(current))
	}
	if kind == "district" {
		d, x := s.GetDistrict(ctx, id)
		if x != nil {
			return BoundaryResult{}, x
		}
		parent, x := s.regionBoundaries.GetGeometry(ctx, d.RegionID)
		if x != nil || parent == nil || !parent.ContainsGeometry(next) {
			return BoundaryResult{}, apierr.New(apierr.InvalidCoordinate, "District boundary must be contained by its region boundary.")
		}
	}
	before, _ := json.Marshal(current)
	after, _ := json.Marshal(next)
	evidence, err := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: a.ID, Label: a.Email, IP: a.IP}, audit.ActionRecordUpdated, audit.Target{Kind: kind + "_geometry", ID: id})
	if err != nil {
		return BoundaryResult{}, apierr.Wrap(apierr.Internal, "Could not create audit evidence.", err)
	}
	evidence = evidence.WithChange(map[string]any{"geometry": string(before)}, map[string]any{"geometry": string(after)}).WithRequest(a.RequestID)
	ok, err := repo.SetGeometryCAS(ctx, id, current, next, evidence)
	if err != nil {
		return BoundaryResult{}, apierr.Wrap(apierr.Internal, "Could not save boundary geometry.", err)
	}
	if !ok {
		return BoundaryResult{}, apierr.New(apierr.Conflict, "Boundary changed while it was being saved.")
	}
	return BoundaryResult{Geometry: next, ETag: boundaryETag(next)}, nil
}

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
		"datasetVersion": r.DatasetVersion,
	}
}

func districtState(d domain.District) map[string]any {
	return map[string]any{
		"name": d.Name, "regionId": d.RegionID, "type": d.DistrictType,
		"code": d.OfficialCode, "capital": d.Capital,
		"status": string(d.Status), "verificationStatus": string(d.VerificationStatus),
		"datasetVersion": d.DatasetVersion,
	}
}

func placeState(p domain.Place) map[string]any {
	return map[string]any{
		"name": p.Name, "type": string(p.Type), "regionId": p.RegionID,
		"districtId": p.DistrictID, "status": string(p.Status),
		"verificationStatus": string(p.VerificationStatus),
		"datasetVersion":     p.DatasetVersion,
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

	if s.admin == nil {
		return nil, apierr.New(apierr.Internal, "Admin geography writes are not configured.")
	}
	evidence, _ := s.mutationEvidence(a, audit.ActionRecordUpdated, "region", id, before, regionState(next))
	err = s.admin.UpdateRegion(ctx, next, evidence)
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

	if s.admin == nil {
		return nil, apierr.New(apierr.Internal, "Admin geography writes are not configured.")
	}
	evidence, _ := s.mutationEvidence(a, audit.ActionRecordUpdated, "district", id, before, districtState(next))
	err = s.admin.UpdateDistrict(ctx, next, evidence)
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

	if s.admin == nil {
		return nil, apierr.New(apierr.Internal, "Admin geography writes are not configured.")
	}
	evidence, _ := s.mutationEvidence(a, audit.ActionRecordUpdated, "place", id, before, placeState(next))
	err = s.admin.UpdatePlace(ctx, next, evidence)
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
