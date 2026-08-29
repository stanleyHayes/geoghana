package graphql

// Hydration of nested references.
//
// A list query returns each record carrying only the ID and NAME of its
// parent, so a caller who never asks for the parent pays nothing for it. When
// they do ask, the full record is loaded here through the per-request loader.
//
// DatasetVersion is the marker for "already complete": every fully-loaded
// record has one, and a stub never does.

import (
	"context"

	"github.com/ghanageo/ghanageo/services/api/internal/transport/graphql/model"
)

func hydrateRegion(ctx context.Context, stub *model.Region) (*model.Region, error) {
	if stub == nil || stub.ID == "" {
		return nil, nil
	}
	if stub.DatasetVersion != "" {
		return stub, nil
	}
	l := loadersFrom(ctx)
	if l == nil {
		return stub, nil
	}
	full, err := l.Region(ctx, stub.ID)
	if err != nil || full == nil {
		// A dangling reference is data worth reporting, not a reason to fail
		// the whole query. The stub still carries the id and name, which is
		// more useful to a caller than null.
		return stub, nil
	}
	return regionOut(*full), nil
}

func hydrateDistrict(ctx context.Context, stub *model.District) (*model.District, error) {
	if stub == nil || stub.ID == "" {
		return nil, nil
	}
	if stub.DatasetVersion != "" {
		return stub, nil
	}
	l := loadersFrom(ctx)
	if l == nil {
		return stub, nil
	}
	full, err := l.District(ctx, stub.ID)
	if err != nil || full == nil {
		return stub, nil
	}
	return districtOut(*full), nil
}
