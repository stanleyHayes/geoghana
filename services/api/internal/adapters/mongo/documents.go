package mongo

import (
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/normalize"
)

// The bson tags live here and nowhere else. The domain stays persistence-free,
// so the storage shape can change without touching domain rules.

type geoJSON struct {
	Type        string `bson:"type"`
	Coordinates any    `bson:"coordinates"`
}

type provenanceDoc struct {
	SourceID          string `bson:"sourceId"`
	ExternalID        string `bson:"externalId,omitempty"`
	SourceURL         string `bson:"sourceUrl,omitempty"`
	RetrievedAt       string `bson:"retrievedAt,omitempty"`
	SourcePayloadHash string `bson:"sourcePayloadHash,omitempty"`
	Notes             string `bson:"notes,omitempty"`
}

type regionDoc struct {
	ID                 string        `bson:"_id"`
	CountryCode        string        `bson:"countryCode"`
	Name               string        `bson:"name"`
	NormalizedName     string        `bson:"normalizedName"`
	Capital            string        `bson:"capital,omitempty"`
	OfficialCode       string        `bson:"officialCode,omitempty"`
	Status             string        `bson:"status"`
	VerificationStatus string        `bson:"verificationStatus"`
	Centroid           *geoJSON      `bson:"centroid,omitempty"`
	Geometry           *geoJSON      `bson:"geometry,omitempty"`
	Provenance         provenanceDoc `bson:"provenance"`
	DatasetVersion     string        `bson:"datasetVersion,omitempty"`
}

type districtDoc struct {
	ID                 string        `bson:"_id"`
	RegionID           string        `bson:"regionId"`
	RegionName         string        `bson:"regionName,omitempty"`
	Name               string        `bson:"name"`
	NormalizedName     string        `bson:"normalizedName"`
	DistrictType       string        `bson:"districtType,omitempty"`
	OfficialCode       string        `bson:"officialCode,omitempty"`
	Capital            string        `bson:"capital,omitempty"`
	Status             string        `bson:"status"`
	VerificationStatus string        `bson:"verificationStatus"`
	Centroid           *geoJSON      `bson:"centroid,omitempty"`
	Geometry           *geoJSON      `bson:"geometry,omitempty"`
	Provenance         provenanceDoc `bson:"provenance"`
	DatasetVersion     string        `bson:"datasetVersion,omitempty"`
}

type aliasDoc struct {
	Value           string `bson:"value"`
	NormalizedValue string `bson:"normalizedValue"`
	AliasType       string `bson:"aliasType,omitempty"`
	Language        string `bson:"language,omitempty"`
	IsPreferred     bool   `bson:"isPreferred,omitempty"`
}

type placeDoc struct {
	ID                 string        `bson:"_id"`
	Name               string        `bson:"name"`
	NormalizedName     string        `bson:"normalizedName"`
	Type               string        `bson:"type"`
	RegionID           string        `bson:"regionId,omitempty"`
	RegionName         string        `bson:"regionName,omitempty"`
	DistrictID         string        `bson:"districtId,omitempty"`
	DistrictName       string        `bson:"districtName,omitempty"`
	ParentPlaceID      string        `bson:"parentPlaceId,omitempty"`
	Aliases            []aliasDoc    `bson:"aliases,omitempty"`
	Population         *int64        `bson:"population,omitempty"`
	Status             string        `bson:"status"`
	VerificationStatus string        `bson:"verificationStatus"`
	Centroid           *geoJSON      `bson:"centroid,omitempty"`
	Geometry           *geoJSON      `bson:"geometry,omitempty"`
	Provenance         provenanceDoc `bson:"provenance"`
	DatasetVersion     string        `bson:"datasetVersion,omitempty"`
}

// ---- domain -> document ----

func pointOf(c *geography.Coordinate) *geoJSON {
	if c == nil {
		return nil
	}
	// GeoJSON position order is [longitude, latitude].
	return &geoJSON{Type: "Point", Coordinates: []float64{c.Longitude, c.Latitude}}
}

func coordOf(g *geoJSON) *geography.Coordinate {
	if g == nil || g.Type != "Point" {
		return nil
	}
	// The driver decodes a BSON array into bson.A, which is a NAMED type
	// (`type A []interface{}`). A type switch on `[]any` does not match a
	// named type, so handling only []any silently returned nil for every
	// coordinate while the database held them correctly.
	switch v := g.Coordinates.(type) {
	case bson.A:
		return coordFromSlice([]any(v))
	case []any:
		return coordFromSlice(v)
	case []float64:
		if len(v) < 2 {
			return nil
		}
		return &geography.Coordinate{Latitude: v[1], Longitude: v[0]}
	}
	return nil
}

// coordFromSlice reads a GeoJSON position, which is [longitude, latitude].
func coordFromSlice(v []any) *geography.Coordinate {
	if len(v) < 2 {
		return nil
	}
	lng, ok1 := toFloat(v[0])
	lat, ok2 := toFloat(v[1])
	if !ok1 || !ok2 {
		return nil
	}
	return &geography.Coordinate{Latitude: lat, Longitude: lng}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}

func geomOf(g *geography.Geometry) *geoJSON {
	if g == nil {
		return nil
	}
	return &geoJSON{Type: string(g.Type), Coordinates: g.Coordinates}
}

// geomFrom is the READ direction for boundary geometry.
//
// Its absence was a silent data loss: geomOf existed, so writes stored
// polygons correctly, but every from*Doc built a domain record with a nil
// Geometry. Boundaries were in the database and invisible to the whole domain.
// /boundaries/{id} masked it by querying the collection directly, so the only
// place it surfaced was a GeoJSON export full of null geometries.
func geomFrom(g *geoJSON) *geography.Geometry {
	if g == nil {
		return nil
	}
	return &geography.Geometry{
		Type:        geography.GeometryType(g.Type),
		Coordinates: g.Coordinates,
	}
}

func provOf(p geography.Provenance) provenanceDoc {
	return provenanceDoc{
		SourceID: p.SourceID, ExternalID: p.ExternalID, SourceURL: p.SourceURL,
		RetrievedAt: p.RetrievedAt, SourcePayloadHash: p.SourcePayloadHash, Notes: p.Notes,
	}
}

func provFrom(p provenanceDoc) geography.Provenance {
	return geography.Provenance{
		SourceID: p.SourceID, ExternalID: p.ExternalID, SourceURL: p.SourceURL,
		RetrievedAt: p.RetrievedAt, SourcePayloadHash: p.SourcePayloadHash, Notes: p.Notes,
	}
}

func toRegionDoc(r geography.Region) regionDoc {
	return regionDoc{
		ID: r.ID, CountryCode: orDefault(r.CountryCode, "GH"), Name: r.Name,
		NormalizedName: normalize.Name(r.Name), Capital: r.Capital, OfficialCode: r.OfficialCode,
		Status: string(r.Status), VerificationStatus: string(r.VerificationStatus),
		Centroid: pointOf(r.Centroid), Geometry: geomOf(r.Geometry),
		Provenance: provOf(r.Provenance), DatasetVersion: r.DatasetVersion,
	}
}

func fromRegionDoc(d regionDoc) geography.Region {
	return geography.Region{
		ID: d.ID, CountryCode: d.CountryCode, Name: d.Name, Capital: d.Capital,
		OfficialCode: d.OfficialCode, Status: geography.Status(d.Status),
		VerificationStatus: geography.VerificationStatus(d.VerificationStatus),
		Centroid:           coordOf(d.Centroid), Geometry: geomFrom(d.Geometry),
		Provenance:     provFrom(d.Provenance),
		DatasetVersion: d.DatasetVersion,
	}
}

func toDistrictDoc(x geography.District) districtDoc {
	return districtDoc{
		ID: x.ID, RegionID: x.RegionID, RegionName: x.RegionName, Name: x.Name,
		NormalizedName: normalize.Name(x.Name), DistrictType: x.DistrictType,
		OfficialCode: x.OfficialCode, Capital: x.Capital, Status: string(x.Status),
		VerificationStatus: string(x.VerificationStatus),
		Centroid:           pointOf(x.Centroid), Geometry: geomOf(x.Geometry),
		Provenance: provOf(x.Provenance), DatasetVersion: x.DatasetVersion,
	}
}

func fromDistrictDoc(d districtDoc) geography.District {
	return geography.District{
		ID: d.ID, RegionID: d.RegionID, RegionName: d.RegionName, Name: d.Name,
		DistrictType: d.DistrictType, OfficialCode: d.OfficialCode, Capital: d.Capital,
		Status: geography.Status(d.Status), VerificationStatus: geography.VerificationStatus(d.VerificationStatus),
		Centroid: coordOf(d.Centroid), Geometry: geomFrom(d.Geometry),
		Provenance:     provFrom(d.Provenance),
		DatasetVersion: d.DatasetVersion,
	}
}

func toPlaceDoc(p geography.Place) placeDoc {
	aliases := make([]aliasDoc, 0, len(p.Aliases))
	for _, a := range p.Aliases {
		aliases = append(aliases, aliasDoc{
			Value: a.Value, NormalizedValue: normalize.Name(a.Value),
			AliasType: a.AliasType, Language: a.Language, IsPreferred: a.IsPreferred,
		})
	}
	return placeDoc{
		ID: p.ID, Name: p.Name, NormalizedName: normalize.Name(p.Name), Type: string(p.Type),
		RegionID: p.RegionID, RegionName: p.RegionName, DistrictID: p.DistrictID,
		DistrictName: p.DistrictName, ParentPlaceID: p.ParentPlaceID, Aliases: aliases,
		Population: p.Population, Status: string(p.Status),
		VerificationStatus: string(p.VerificationStatus),
		Centroid:           pointOf(p.Centroid), Geometry: geomOf(p.Geometry),
		Provenance: provOf(p.Provenance), DatasetVersion: p.DatasetVersion,
	}
}

func fromPlaceDoc(d placeDoc) geography.Place {
	aliases := make([]geography.Alias, 0, len(d.Aliases))
	for _, a := range d.Aliases {
		aliases = append(aliases, geography.Alias{
			Value: a.Value, NormalizedValue: a.NormalizedValue,
			AliasType: a.AliasType, Language: a.Language, IsPreferred: a.IsPreferred,
		})
	}
	return geography.Place{
		ID: d.ID, Name: d.Name, NormalizedName: d.NormalizedName,
		Type: geography.PlaceType(d.Type), RegionID: d.RegionID, RegionName: d.RegionName,
		DistrictID: d.DistrictID, DistrictName: d.DistrictName, ParentPlaceID: d.ParentPlaceID,
		Aliases: aliases, Population: d.Population, Status: geography.Status(d.Status),
		VerificationStatus: geography.VerificationStatus(d.VerificationStatus),
		Centroid:           coordOf(d.Centroid), Geometry: geomFrom(d.Geometry),
		Provenance:     provFrom(d.Provenance),
		DatasetVersion: d.DatasetVersion,
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
