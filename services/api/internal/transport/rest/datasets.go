package rest

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/dataset"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

// Dataset catalogue endpoints (Spec §7.2, story GEO-8.3).
//
// Bulk download is how a consumer avoids hammering the API for data that does
// not change between releases, so it is a first-class endpoint rather than a
// convenience: it is cheaper for them AND for us.

type artifactJSON struct {
	Entity      string `json:"entity"`
	Format      string `json:"format"`
	SizeBytes   int64  `json:"sizeBytes"`
	SHA256      string `json:"sha256"`
	RecordCount int64  `json:"recordCount"`
	URL         string `json:"url"`
}

type datasetJSON struct {
	Version     string           `json:"version"`
	Status      string           `json:"status"`
	PublishedAt string           `json:"publishedAt,omitempty"`
	Changelog   string           `json:"changelog,omitempty"`
	Counts      map[string]int64 `json:"counts,omitempty"`
	Downloads   []artifactJSON   `json:"downloads"`
	Licence     string           `json:"license"`
	Attribution string           `json:"attribution"`
}

func toDatasetJSON(v domain.Version) datasetJSON {
	downloads := make([]artifactJSON, 0, len(v.Artifacts))
	for _, a := range v.Artifacts {
		downloads = append(downloads, artifactJSON{
			Entity: a.Entity, Format: string(a.Format),
			SizeBytes: a.SizeBytes, SHA256: a.SHA256, RecordCount: a.RecordCount,
			URL: fmt.Sprintf("/v1/datasets/%s/downloads/%s.%s", v.Version, a.Entity, a.Format),
		})
	}
	return datasetJSON{
		Version: v.Version, Status: string(v.Status), PublishedAt: v.PublishedAt,
		Changelog: v.Changelog, Counts: v.Counts, Downloads: downloads,
		// Attribution is on every response, not only on the file, because a
		// client that only reads the catalogue still needs to know the terms.
		Licence: domain.Licence, Attribution: domain.Attribution,
	}
}

func (h *Handler) listDatasets(w http.ResponseWriter, r *http.Request) {
	if h.datasets == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Dataset catalogue is not configured."))
		return
	}
	versions, err := h.datasets.List(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]datasetJSON, 0, len(versions))
	for _, v := range versions {
		out = append(out, toDatasetJSON(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) datasetDownloads(w http.ResponseWriter, r *http.Request) {
	if h.datasets == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Dataset catalogue is not configured."))
		return
	}
	v, err := h.datasets.Downloads(r.Context(), chi.URLParam(r, "version"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": toDatasetJSON(*v)})
}

// datasetArtifact streams one file.
//
// The path parameter is "<entity>.<format>" — matched against the RECORDED
// artifact list, never used to build a filesystem path directly.
func (h *Handler) datasetArtifact(w http.ResponseWriter, r *http.Request) {
	if h.datasets == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Dataset catalogue is not configured."))
		return
	}
	version := chi.URLParam(r, "version")
	entity := chi.URLParam(r, "entity")
	format := chi.URLParam(r, "format")

	f, a, err := h.datasets.OpenArtifact(r.Context(), version, entity, format)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer func() { _ = f.Close() }()

	w.Header().Set("Content-Type", a.Format.ContentType())
	w.Header().Set("Content-Length", strconv.FormatInt(a.SizeBytes, 10))
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="ghanageo-%s-%s"`, version, a.Filename))
	// The checksum a client should verify against, so a truncated transfer is
	// detectable without re-downloading.
	w.Header().Set("X-Checksum-SHA256", a.SHA256)
	w.Header().Set("X-Attribution", domain.Attribution)
	// A published version is immutable, so it can be cached hard.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	// ServeContent handles range requests and conditional GETs, which matters
	// for the largest artifact on a poor connection. A zero modtime disables
	// If-Modified-Since; the immutable Cache-Control and the checksum are the
	// freshness contract, and a published version never changes.
	http.ServeContent(w, r, a.Filename, time.Time{}, f)
}
