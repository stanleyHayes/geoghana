package grpc

import (
	"testing"

	pb "github.com/ghanageo/ghanageo/services/api/gen/ghanageo/v1"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
	"google.golang.org/grpc/codes"
)

// Every domain enum value must map to a NAMED protobuf value. A missing entry
// silently becomes the zero value — a CANONICAL record would go out as
// UNSPECIFIED, which is a data-integrity bug no compiler catches.
func TestEveryDomainEnumValueMaps(t *testing.T) {
	for _, s := range []domain.Status{
		domain.StatusActive, domain.StatusDeprecated, domain.StatusMerged,
	} {
		if got := statusToPB[s]; got == pb.Status_STATUS_UNSPECIFIED {
			t.Errorf("Status %q maps to UNSPECIFIED", s)
		}
	}
	for _, v := range []domain.VerificationStatus{
		domain.VerificationReference, domain.VerificationNeedsRecon,
		domain.VerificationReviewed, domain.VerificationCanonical,
	} {
		if got := verificationToPB[v]; got == pb.VerificationStatus_VERIFICATION_STATUS_UNSPECIFIED {
			t.Errorf("VerificationStatus %q maps to UNSPECIFIED", v)
		}
	}
	for _, p := range []domain.PlaceType{
		domain.PlaceCity, domain.PlaceTown, domain.PlaceVillage, domain.PlaceCommunity,
		domain.PlaceSuburb, domain.PlaceNeighbourhood, domain.PlaceHamlet,
		domain.PlaceSettlement, domain.PlaceLocality, domain.PlaceRegionalCapital,
	} {
		if got := placeTypeToPB[p]; got == pb.PlaceType_PLACE_TYPE_UNSPECIFIED {
			t.Errorf("PlaceType %q maps to UNSPECIFIED", p)
		}
	}
}

// The PlaceType round trip must be lossless, or a type filter sent over gRPC
// would quietly select a different type than the caller asked for.
func TestPlaceTypeRoundTrip(t *testing.T) {
	for dt, pt := range placeTypeToPB {
		if got := placeTypeFromPB(pt); got != string(dt) {
			t.Errorf("round trip %q: got %q", dt, got)
		}
	}
	// UNSPECIFIED means "no filter", not "invalid".
	if got := placeTypeFromPB(pb.PlaceType_PLACE_TYPE_UNSPECIFIED); got != "" {
		t.Errorf("UNSPECIFIED should mean no filter, got %q", got)
	}
}

// Every code in the shared catalog needs a gRPC code. A missing entry falls
// back to INTERNAL, which would tell a client to retry a permanent failure.
func TestEveryErrorCodeMapsToGRPC(t *testing.T) {
	all := []apierr.Code{
		apierr.InvalidArgument, apierr.InvalidCoordinate, apierr.RadiusOutOfRange,
		apierr.QueryTooShort, apierr.PayloadTooLarge, apierr.Unauthenticated,
		apierr.KeyRevoked, apierr.PermissionDenied, apierr.OriginNotAllowed,
		apierr.NotFound, apierr.ResourceGone, apierr.RateLimitExceeded,
		apierr.QuotaExceeded, apierr.QueryTooComplex, apierr.DeadlineExceeded,
		apierr.Internal,
	}
	for _, c := range all {
		got, ok := grpcCode[c]
		if !ok {
			t.Errorf("error code %q has no gRPC mapping", c)
			continue
		}
		// Only INTERNAL may map to codes.Internal; anything else doing so
		// would misreport a client error as a server fault.
		if got == codes.Internal && c != apierr.Internal {
			t.Errorf("error code %q maps to INTERNAL", c)
		}
	}
}

func TestClampLimit(t *testing.T) {
	cases := []struct{ in, want int32 }{
		{0, defaultLimit},  // proto3 zero means unset, not "zero results"
		{-5, defaultLimit}, // a negative limit is nonsense, not an error
		{10, 10},
		{maxLimit, maxLimit},
		{maxLimit + 1, maxLimit},
		{100_000, maxLimit},
	}
	for _, c := range cases {
		if got := clampLimit(c.in); got != int(c.want) {
			t.Errorf("clampLimit(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

// An unassigned district must be ABSENT, not a Ref with two empty strings —
// a client checking `district != nil` would otherwise see a phantom record.
func TestRefOmittedWhenNoID(t *testing.T) {
	if got := refToPB("", "Some Name"); got != nil {
		t.Errorf("expected nil Ref for empty id, got %+v", got)
	}
	if got := refToPB("gh-region-ashanti", "Ashanti"); got == nil {
		t.Fatal("expected a Ref for a real id")
	}
}

// Aliases must keep their true orthography on the wire. Normalization exists
// for matching; sending the folded form would corrupt Twi, Ga and Ewe names.
func TestAliasKeepsOrthography(t *testing.T) {
	p := domain.Place{
		ID:   "gh-place-osu",
		Name: "Ɔsu",
		Aliases: []domain.Alias{
			{Value: "Ɔsu", NormalizedValue: "osu", AliasType: "endonym", Language: "gaa"},
		},
	}
	got := placeToPB(p)
	if got.GetName() != "Ɔsu" {
		t.Errorf("name folded: %q", got.GetName())
	}
	if len(got.GetAliases()) != 1 || got.GetAliases()[0].GetValue() != "Ɔsu" {
		t.Errorf("alias folded: %+v", got.GetAliases())
	}
}

// A search doc is a projection with no provenance. It must map to zero values
// rather than to invented ones.
func TestSearchHitCarriesNoInventedProvenance(t *testing.T) {
	got := searchHitToPB(ports.SearchHit{
		Doc:   ports.SearchDoc{ID: "gh-place-kumasi", Name: "Kumasi", Type: "CITY"},
		Score: 0.97, MatchReason: "exact",
	})
	if got.GetPlace().GetProvenance() != nil {
		t.Error("search projection must not fabricate provenance")
	}
	if got.GetScore() != 0.97 || got.GetMatchReason() != "exact" {
		t.Errorf("score/reason lost: %+v", got)
	}
	// No coordinates on the doc means no centroid, not 0,0 — which is in the
	// Gulf of Guinea, roughly 380km off the coast of Ghana.
	if got.GetPlace().GetCentroid() != nil {
		t.Error("absent coordinates became a centroid at null island")
	}
}
