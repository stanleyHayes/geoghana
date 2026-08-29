package rest

import (
	"net/http"

	appsearch "github.com/ghanageo/ghanageo/services/api/internal/app/search"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

type searchResultDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Kind         string  `json:"kind"`
	RegionID     string  `json:"regionId,omitempty"`
	RegionName   string  `json:"regionName,omitempty"`
	DistrictID   string  `json:"districtId,omitempty"`
	DistrictName string  `json:"districtName,omitempty"`
	Score        float64 `json:"score"`
	MatchReason  string  `json:"matchReason,omitempty"`
}

func searchOut(h ports.SearchHit) searchResultDTO {
	return searchResultDTO{
		ID: h.Doc.ID, Name: h.Doc.Name, Type: h.Doc.Type, Kind: h.Doc.Kind,
		RegionID: h.Doc.RegionID, RegionName: h.Doc.RegionName,
		DistrictID: h.Doc.DistrictID, DistrictName: h.Doc.DistrictName,
		Score: h.Score, MatchReason: h.MatchReason,
	}
}

func (h *Handler) writeHits(w http.ResponseWriter, r *http.Request, hits []ports.SearchHit) {
	out := make([]searchResultDTO, 0, len(hits))
	for _, hit := range hits {
		out = append(out, searchOut(hit))
	}
	writeJSON(w, http.StatusOK, pageDTO[searchResultDTO]{
		Data: out, DatasetVersion: h.search.DatasetVersion(),
	})
}

func (h *Handler) searchHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	hits, err := h.search.Search(r.Context(), ports.SearchQuery{
		Text:       q.Get("q"),
		Limit:      intParam(r, "limit", 20),
		RegionID:   q.Get("regionId"),
		DistrictID: q.Get("districtId"),
		Type:       q.Get("type"),
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	h.writeHits(w, r, hits)
}

func (h *Handler) autocompleteHandler(w http.ResponseWriter, r *http.Request) {
	hits, err := h.search.Autocomplete(r.Context(), r.URL.Query().Get("q"), intParam(r, "limit", 10))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	h.writeHits(w, r, hits)
}

func (h *Handler) geocodeHandler(w http.ResponseWriter, r *http.Request) {
	hits, err := h.search.Geocode(r.Context(), r.URL.Query().Get("q"), intParam(r, "limit", 10))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	h.writeHits(w, r, hits)
}

type reverseDTO struct {
	Region         *refDTO    `json:"region,omitempty"`
	District       *refDTO    `json:"district,omitempty"`
	Nearby         []placeDTO `json:"nearby"`
	DatasetVersion string     `json:"datasetVersion"`
}

func (h *Handler) reverseHandler(w http.ResponseWriter, r *http.Request) {
	lat, okLat := floatParam(r, "lat")
	lng, okLng := floatParam(r, "lng")
	if !okLat || !okLng {
		writeErr(w, r, apierr.New(apierr.InvalidArgument, "Both lat and lng are required."))
		return
	}
	res, err := h.search.Reverse(r.Context(), lat, lng)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := reverseDTO{Nearby: make([]placeDTO, 0, len(res.Nearby)), DatasetVersion: res.DatasetVersion}
	if res.Region != nil {
		out.Region = &refDTO{ID: res.Region.ID, Name: res.Region.Name}
	}
	if res.District != nil {
		out.District = &refDTO{ID: res.District.ID, Name: res.District.Name}
	}
	for _, p := range res.Nearby {
		out.Nearby = append(out.Nearby, placeOut(p))
	}
	writeJSON(w, http.StatusOK, out)
}

var _ = appsearch.MinQueryLength
