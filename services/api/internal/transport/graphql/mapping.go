package graphql

import (
	"math"

	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
	"github.com/ghanageo/ghanageo/services/api/internal/transport/graphql/model"
)

// Mapping between the domain and the generated GraphQL models.
//
// The transport owns this translation; the domain knows nothing about GraphQL,
// exactly as it knows nothing about BSON or JSON.

func coord(c *domain.Coordinate) *model.Coordinate {
	if c == nil {
		return nil
	}
	return &model.Coordinate{Latitude: c.Latitude, Longitude: c.Longitude}
}

func provenance(p domain.Provenance) *model.Provenance {
	out := &model.Provenance{SourceID: p.SourceID}
	if p.SourceURL != "" {
		u := p.SourceURL
		out.SourceURL = &u
	}
	if p.RetrievedAt != "" {
		r := p.RetrievedAt
		out.RetrievedAt = &r
	}
	return out
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func regionOut(r domain.Region) *model.Region {
	return &model.Region{
		ID:                 r.ID,
		Code:               strPtr(r.OfficialCode),
		Name:               r.Name,
		CountryCode:        orDefault(r.CountryCode, "GH"),
		Capital:            strPtr(r.Capital),
		Centroid:           coord(r.Centroid),
		Status:             model.Status(r.Status),
		VerificationStatus: model.VerificationStatus(r.VerificationStatus),
		Provenance:         provenance(r.Provenance),
		DatasetVersion:     r.DatasetVersion,
	}
}

func districtOut(d domain.District) *model.District {
	return &model.District{
		ID:      d.ID,
		Code:    strPtr(d.OfficialCode),
		Name:    d.Name,
		Type:    strPtr(d.DistrictType),
		Capital: strPtr(d.Capital),
		// Region is resolved lazily by the District.region field resolver, so a
		// query that does not ask for it costs nothing.
		Region:             &model.Region{ID: d.RegionID, Name: d.RegionName},
		Centroid:           coord(d.Centroid),
		Status:             model.Status(d.Status),
		VerificationStatus: model.VerificationStatus(d.VerificationStatus),
		Provenance:         provenance(d.Provenance),
		DatasetVersion:     d.DatasetVersion,
	}
}

func placeOut(p domain.Place) *model.Place {
	// gqlgen.yml sets omit_slice_element_pointers, so list elements are values.
	aliases := make([]model.PlaceAlias, 0, len(p.Aliases))
	for _, a := range p.Aliases {
		aliases = append(aliases, model.PlaceAlias{
			Value:       a.Value,
			Type:        strPtr(a.AliasType),
			Language:    strPtr(a.Language),
			IsPreferred: a.IsPreferred,
		})
	}
	out := &model.Place{
		ID:                 p.ID,
		Name:               p.Name,
		NormalizedName:     p.NormalizedName,
		Type:               model.PlaceType(p.Type),
		Aliases:            aliases,
		Centroid:           coord(p.Centroid),
		Status:             model.Status(p.Status),
		VerificationStatus: model.VerificationStatus(p.VerificationStatus),
		Provenance:         provenance(p.Provenance),
		DatasetVersion:     p.DatasetVersion,
	}
	if p.RegionID != "" {
		out.Region = &model.Region{ID: p.RegionID, Name: p.RegionName}
	}
	if p.DistrictID != "" {
		out.District = &model.District{ID: p.DistrictID, Name: p.DistrictName}
	}
	if p.Population != nil {
		n := int(*p.Population)
		out.Population = &n
	}
	return out
}

func pageInfo(nextCursor string) *model.PageInfo {
	pi := &model.PageInfo{HasNextPage: nextCursor != ""}
	if nextCursor != "" {
		c := nextCursor
		pi.EndCursor = &c
	}
	return pi
}

func regionConnection(p ports.Page[domain.Region]) *model.RegionConnection {
	nodes := make([]model.Region, 0, len(p.Data))
	edges := make([]model.RegionEdge, 0, len(p.Data))
	for _, r := range p.Data {
		n := regionOut(r)
		nodes = append(nodes, *n)
		edges = append(edges, model.RegionEdge{Node: n, Cursor: r.ID})
	}
	return &model.RegionConnection{
		Edges: edges, Nodes: nodes,
		PageInfo: pageInfo(p.NextCursor), DatasetVersion: p.DatasetVersion,
	}
}

func districtConnection(p ports.Page[domain.District]) *model.DistrictConnection {
	nodes := make([]model.District, 0, len(p.Data))
	edges := make([]model.DistrictEdge, 0, len(p.Data))
	for _, d := range p.Data {
		n := districtOut(d)
		nodes = append(nodes, *n)
		edges = append(edges, model.DistrictEdge{Node: n, Cursor: d.ID})
	}
	return &model.DistrictConnection{
		Edges: edges, Nodes: nodes,
		PageInfo: pageInfo(p.NextCursor), DatasetVersion: p.DatasetVersion,
	}
}

func placeConnection(p ports.Page[domain.Place]) *model.PlaceConnection {
	nodes := make([]model.Place, 0, len(p.Data))
	edges := make([]model.PlaceEdge, 0, len(p.Data))
	for _, x := range p.Data {
		n := placeOut(x)
		nodes = append(nodes, *n)
		edges = append(edges, model.PlaceEdge{Node: n, Cursor: x.ID})
	}
	return &model.PlaceConnection{
		Edges: edges, Nodes: nodes,
		PageInfo: pageInfo(p.NextCursor), DatasetVersion: p.DatasetVersion,
	}
}

func limitOf(first *int, def int) int {
	if first == nil || *first <= 0 {
		return def
	}
	return *first
}

func strOf(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func searchResults(hits []ports.SearchHit) []model.SearchResult {
	out := make([]model.SearchResult, 0, len(hits))
	for _, h := range hits {
		out = append(out, model.SearchResult{
			Place: &model.Place{
				ID:             h.Doc.ID,
				Name:           h.Doc.Name,
				NormalizedName: h.Doc.Normalized,
				Type:           model.PlaceType(h.Doc.Type),
				Aliases:        []model.PlaceAlias{},
				Region:         &model.Region{ID: h.Doc.RegionID, Name: h.Doc.RegionName},
				Status:         model.StatusActive,
			},
			Score:       h.Score,
			MatchReason: strPtr(h.MatchReason),
		})
	}
	return out
}

func searchConnection(hits []ports.SearchHit, version string) *model.SearchConnection {
	return &model.SearchConnection{
		Nodes:          searchResults(hits),
		PageInfo:       pageInfo(""),
		DatasetVersion: version,
	}
}

// haversineMeters is great-circle distance on a sphere. MongoDB's $nearSphere
// orders by distance but does not return it, and re-querying with $geoNear
// purely to obtain a number would cost a second aggregation per request.
func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusM = 6371000.0
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := rad(lat2 - lat1)
	dLon := rad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
