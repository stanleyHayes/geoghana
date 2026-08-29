package grpc

import (
	pb "github.com/ghanageo/ghanageo/services/api/gen/ghanageo/v1"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

// Domain to protobuf. The reverse direction is deliberately absent: gRPC is a
// read interface, so nothing here ever constructs a domain record from wire
// input beyond scalar request fields.

var statusToPB = map[domain.Status]pb.Status{
	domain.StatusActive:     pb.Status_STATUS_ACTIVE,
	domain.StatusDeprecated: pb.Status_STATUS_DEPRECATED,
	domain.StatusMerged:     pb.Status_STATUS_MERGED,
}

var verificationToPB = map[domain.VerificationStatus]pb.VerificationStatus{
	domain.VerificationReference: pb.VerificationStatus_VERIFICATION_STATUS_REFERENCE,
	domain.VerificationNeedsRecon: pb.
		VerificationStatus_VERIFICATION_STATUS_SEED_NEEDS_CANONICAL_RECONCILIATION,
	domain.VerificationReviewed:  pb.VerificationStatus_VERIFICATION_STATUS_REVIEWED,
	domain.VerificationCanonical: pb.VerificationStatus_VERIFICATION_STATUS_CANONICAL,
}

var placeTypeToPB = map[domain.PlaceType]pb.PlaceType{
	domain.PlaceCity:            pb.PlaceType_PLACE_TYPE_CITY,
	domain.PlaceTown:            pb.PlaceType_PLACE_TYPE_TOWN,
	domain.PlaceVillage:         pb.PlaceType_PLACE_TYPE_VILLAGE,
	domain.PlaceCommunity:       pb.PlaceType_PLACE_TYPE_COMMUNITY,
	domain.PlaceSuburb:          pb.PlaceType_PLACE_TYPE_SUBURB,
	domain.PlaceNeighbourhood:   pb.PlaceType_PLACE_TYPE_NEIGHBOURHOOD,
	domain.PlaceHamlet:          pb.PlaceType_PLACE_TYPE_HAMLET,
	domain.PlaceSettlement:      pb.PlaceType_PLACE_TYPE_SETTLEMENT,
	domain.PlaceLocality:        pb.PlaceType_PLACE_TYPE_LOCALITY,
	domain.PlaceRegionalCapital: pb.PlaceType_PLACE_TYPE_REGIONAL_CAPITAL,
}

// placeTypeFromPB converts a request enum back to the domain string. The
// UNSPECIFIED zero value means "no filter", NOT an invalid type — a proto3
// enum field is always present, so a client that omits it must not be told its
// filter was wrong.
func placeTypeFromPB(t pb.PlaceType) string {
	for dt, pt := range placeTypeToPB {
		if pt == t {
			return string(dt)
		}
	}
	return ""
}

func coordinateToPB(c *domain.Coordinate) *pb.Coordinate {
	if c == nil {
		return nil
	}
	return &pb.Coordinate{Latitude: c.Latitude, Longitude: c.Longitude}
}

func provenanceToPB(p domain.Provenance) *pb.Provenance {
	return &pb.Provenance{
		SourceId:    p.SourceID,
		SourceUrl:   p.SourceURL,
		RetrievedAt: p.RetrievedAt,
	}
}

// refToPB returns nil when there is no id, so an unassigned district stays
// absent on the wire rather than becoming a Ref with two empty strings.
func refToPB(id, name string) *pb.Ref {
	if id == "" {
		return nil
	}
	return &pb.Ref{Id: id, Name: name}
}

func regionToPB(r domain.Region) *pb.Region {
	return &pb.Region{
		Id:                 r.ID,
		CountryCode:        r.CountryCode,
		Name:               r.Name,
		Capital:            r.Capital,
		Code:               r.OfficialCode,
		Status:             statusToPB[r.Status],
		VerificationStatus: verificationToPB[r.VerificationStatus],
		Centroid:           coordinateToPB(r.Centroid),
		Provenance:         provenanceToPB(r.Provenance),
		DatasetVersion:     r.DatasetVersion,
	}
}

func districtToPB(d domain.District) *pb.District {
	return &pb.District{
		Id:                 d.ID,
		Name:               d.Name,
		Code:               d.OfficialCode,
		Type:               d.DistrictType,
		Capital:            d.Capital,
		Region:             refToPB(d.RegionID, d.RegionName),
		Status:             statusToPB[d.Status],
		VerificationStatus: verificationToPB[d.VerificationStatus],
		Centroid:           coordinateToPB(d.Centroid),
		Provenance:         provenanceToPB(d.Provenance),
		DatasetVersion:     d.DatasetVersion,
	}
}

func placeToPB(p domain.Place) *pb.Place {
	aliases := make([]*pb.Alias, 0, len(p.Aliases))
	for _, a := range p.Aliases {
		aliases = append(aliases, &pb.Alias{
			// Value keeps the true orthography; only matching folds it away.
			Value:       a.Value,
			Type:        a.AliasType,
			Language:    a.Language,
			IsPreferred: a.IsPreferred,
		})
	}
	return &pb.Place{
		Id:                 p.ID,
		Name:               p.Name,
		NormalizedName:     p.NormalizedName,
		Type:               placeTypeToPB[p.Type],
		Region:             refToPB(p.RegionID, p.RegionName),
		District:           refToPB(p.DistrictID, p.DistrictName),
		ParentPlaceId:      p.ParentPlaceID,
		Aliases:            aliases,
		Centroid:           coordinateToPB(p.Centroid),
		Population:         p.Population,
		Status:             statusToPB[p.Status],
		VerificationStatus: verificationToPB[p.VerificationStatus],
		Provenance:         provenanceToPB(p.Provenance),
		DatasetVersion:     p.DatasetVersion,
	}
}

// searchHitToPB renders a hit as a Place plus its score and reason.
//
// A search document is a projection, not a full record: it carries no
// provenance or population. Those fields are left zero rather than invented,
// and a client that needs them calls GetPlace with the id.
func searchHitToPB(h ports.SearchHit) *pb.SearchResult {
	d := h.Doc
	var centroid *pb.Coordinate
	if d.Latitude != 0 || d.Longitude != 0 {
		centroid = &pb.Coordinate{Latitude: d.Latitude, Longitude: d.Longitude}
	}
	aliases := make([]*pb.Alias, 0, len(d.Aliases))
	for _, a := range d.Aliases {
		aliases = append(aliases, &pb.Alias{Value: a})
	}
	return &pb.SearchResult{
		Place: &pb.Place{
			Id:             d.ID,
			Name:           d.Name,
			NormalizedName: d.Normalized,
			Type:           placeTypeToPB[domain.PlaceType(d.Type)],
			Region:         refToPB(d.RegionID, d.RegionName),
			District:       refToPB(d.DistrictID, d.DistrictName),
			Aliases:        aliases,
			Centroid:       centroid,
		},
		Score:       h.Score,
		MatchReason: h.MatchReason,
	}
}
