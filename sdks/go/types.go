package ghanageo

import "encoding/json"

type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
type Ref struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Provenance struct {
	SourceID    string `json:"sourceId"`
	SourceURL   string `json:"sourceUrl,omitempty"`
	RetrievedAt string `json:"retrievedAt,omitempty"`
}
type Region struct {
	ID                 string      `json:"id"`
	CountryCode        string      `json:"countryCode"`
	Name               string      `json:"name"`
	Capital            string      `json:"capital,omitempty"`
	Code               string      `json:"code,omitempty"`
	Status             string      `json:"status"`
	VerificationStatus string      `json:"verificationStatus"`
	Centroid           *Coordinate `json:"centroid,omitempty"`
	Provenance         Provenance  `json:"provenance"`
	DatasetVersion     string      `json:"datasetVersion"`
}
type District struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	Code               string      `json:"code,omitempty"`
	Type               string      `json:"type,omitempty"`
	Capital            string      `json:"capital,omitempty"`
	Region             Ref         `json:"region"`
	Status             string      `json:"status"`
	VerificationStatus string      `json:"verificationStatus"`
	Centroid           *Coordinate `json:"centroid,omitempty"`
	Provenance         Provenance  `json:"provenance"`
	DatasetVersion     string      `json:"datasetVersion"`
}
type Alias struct {
	Value       string `json:"value"`
	Type        string `json:"type,omitempty"`
	Language    string `json:"language,omitempty"`
	IsPreferred bool   `json:"isPreferred,omitempty"`
}
type Place struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	NormalizedName     string      `json:"normalizedName"`
	Type               string      `json:"type"`
	Region             *Ref        `json:"region,omitempty"`
	District           *Ref        `json:"district,omitempty"`
	ParentPlaceID      string      `json:"parentPlaceId,omitempty"`
	Aliases            []Alias     `json:"aliases"`
	Centroid           *Coordinate `json:"centroid,omitempty"`
	Population         *int64      `json:"population,omitempty"`
	Status             string      `json:"status"`
	VerificationStatus string      `json:"verificationStatus"`
	Provenance         Provenance  `json:"provenance"`
	DatasetVersion     string      `json:"datasetVersion"`
}
type SearchResult struct {
	Place
	Score       float64 `json:"score,omitempty"`
	MatchReason string  `json:"matchReason,omitempty"`
}
type Page[T any] struct {
	Data           []T    `json:"data"`
	DatasetVersion string `json:"datasetVersion,omitempty"`
	NextCursor     string `json:"nextCursor,omitempty"`
}
type ReverseResult struct {
	Region         *Ref    `json:"region,omitempty"`
	District       *Ref    `json:"district,omitempty"`
	Nearby         []Place `json:"nearby,omitempty"`
	DatasetVersion string  `json:"datasetVersion"`
}
type BoundaryFeature struct {
	Type       string          `json:"type"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties map[string]any  `json:"properties"`
}
type DatasetVersion struct {
	Version     string `json:"version"`
	Status      string `json:"status"`
	PublishedAt string `json:"publishedAt,omitempty"`
	Changelog   string `json:"changelog,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
}
type DatasetPage struct {
	Data []DatasetVersion `json:"data"`
}
type Download struct {
	Format      string `json:"format"`
	URL         string `json:"url"`
	SizeBytes   int64  `json:"sizeBytes,omitempty"`
	Checksum    string `json:"checksum"`
	Attribution string `json:"attribution,omitempty"`
}
type DownloadList struct {
	Version   string     `json:"version"`
	Downloads []Download `json:"downloads"`
}

type PageOptions struct {
	Cursor string
	Limit  int
}
type DistrictOptions struct {
	PageOptions
	RegionID, Query string
}
type PlaceOptions struct {
	PageOptions
	DistrictID, RegionID, Type, Query string
}
type SearchOptions struct {
	RegionID, DistrictID, Type string
	Limit                      int
}
