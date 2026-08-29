// Package rest is a transport adapter. It validates wire format, calls a use
// case, and maps the result back. It contains NO business logic (plan rule R6).
package rest

import (
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

type coordinateDTO struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type refDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type aliasDTO struct {
	Value       string `json:"value"`
	Type        string `json:"type,omitempty"`
	Language    string `json:"language,omitempty"`
	IsPreferred bool   `json:"isPreferred,omitempty"`
}

type provenanceDTO struct {
	SourceID    string `json:"sourceId"`
	SourceURL   string `json:"sourceUrl,omitempty"`
	RetrievedAt string `json:"retrievedAt,omitempty"`
}

type regionDTO struct {
	ID                 string         `json:"id"`
	CountryCode        string         `json:"countryCode"`
	Name               string         `json:"name"`
	Capital            string         `json:"capital,omitempty"`
	Code               string         `json:"code,omitempty"`
	Status             string         `json:"status"`
	VerificationStatus string         `json:"verificationStatus"`
	Centroid           *coordinateDTO `json:"centroid,omitempty"`
	Provenance         provenanceDTO  `json:"provenance"`
	DatasetVersion     string         `json:"datasetVersion"`
}

type districtDTO struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	Code               string         `json:"code,omitempty"`
	Type               string         `json:"type,omitempty"`
	Capital            string         `json:"capital,omitempty"`
	Region             refDTO         `json:"region"`
	Status             string         `json:"status"`
	VerificationStatus string         `json:"verificationStatus"`
	Centroid           *coordinateDTO `json:"centroid,omitempty"`
	Provenance         provenanceDTO  `json:"provenance"`
	DatasetVersion     string         `json:"datasetVersion"`
}

type placeDTO struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	NormalizedName     string         `json:"normalizedName"`
	Type               string         `json:"type"`
	Region             *refDTO        `json:"region,omitempty"`
	District           *refDTO        `json:"district,omitempty"`
	ParentPlaceID      string         `json:"parentPlaceId,omitempty"`
	Aliases            []aliasDTO     `json:"aliases"`
	Centroid           *coordinateDTO `json:"centroid,omitempty"`
	Population         *int64         `json:"population,omitempty"`
	Status             string         `json:"status"`
	VerificationStatus string         `json:"verificationStatus"`
	Provenance         provenanceDTO  `json:"provenance"`
	DatasetVersion     string         `json:"datasetVersion"`
}

// pageDTO is the shared list envelope. Every list response carries the dataset
// version (Spec 18).
type pageDTO[T any] struct {
	Data           []T    `json:"data"`
	NextCursor     string `json:"nextCursor,omitempty"`
	DatasetVersion string `json:"datasetVersion"`
}

func coordOut(c *domain.Coordinate) *coordinateDTO {
	if c == nil {
		return nil
	}
	return &coordinateDTO{Latitude: c.Latitude, Longitude: c.Longitude}
}

func provOut(p domain.Provenance) provenanceDTO {
	return provenanceDTO{SourceID: p.SourceID, SourceURL: p.SourceURL, RetrievedAt: p.RetrievedAt}
}

func regionOut(r domain.Region) regionDTO {
	return regionDTO{
		ID: r.ID, CountryCode: r.CountryCode, Name: r.Name, Capital: r.Capital,
		Code: r.OfficialCode, Status: string(r.Status),
		VerificationStatus: string(r.VerificationStatus), Centroid: coordOut(r.Centroid),
		Provenance: provOut(r.Provenance), DatasetVersion: r.DatasetVersion,
	}
}

func districtOut(d domain.District) districtDTO {
	return districtDTO{
		ID: d.ID, Name: d.Name, Code: d.OfficialCode, Type: d.DistrictType, Capital: d.Capital,
		Region: refDTO{ID: d.RegionID, Name: d.RegionName}, Status: string(d.Status),
		VerificationStatus: string(d.VerificationStatus), Centroid: coordOut(d.Centroid),
		Provenance: provOut(d.Provenance), DatasetVersion: d.DatasetVersion,
	}
}

func placeOut(p domain.Place) placeDTO {
	aliases := make([]aliasDTO, 0, len(p.Aliases))
	for _, a := range p.Aliases {
		aliases = append(aliases, aliasDTO{
			Value: a.Value, Type: a.AliasType, Language: a.Language, IsPreferred: a.IsPreferred,
		})
	}
	out := placeDTO{
		ID: p.ID, Name: p.Name, NormalizedName: p.NormalizedName, Type: string(p.Type),
		ParentPlaceID: p.ParentPlaceID, Aliases: aliases, Centroid: coordOut(p.Centroid),
		Population: p.Population, Status: string(p.Status),
		VerificationStatus: string(p.VerificationStatus),
		Provenance:         provOut(p.Provenance), DatasetVersion: p.DatasetVersion,
	}
	if p.RegionID != "" {
		out.Region = &refDTO{ID: p.RegionID, Name: p.RegionName}
	}
	if p.DistrictID != "" {
		out.District = &refDTO{ID: p.DistrictID, Name: p.DistrictName}
	}
	return out
}

func pageOut[D any, T any](p ports.Page[D], conv func(D) T) pageDTO[T] {
	data := make([]T, 0, len(p.Data))
	for _, d := range p.Data {
		data = append(data, conv(d))
	}
	return pageDTO[T]{Data: data, NextCursor: p.NextCursor, DatasetVersion: p.DatasetVersion}
}
