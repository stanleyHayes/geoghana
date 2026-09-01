// Package client is a thin HTTP client for the public GhanaGeo API.
//
// It carries no authentication requirement: GhanaGeo is free, and anonymous
// requests are a first-class path, not a degraded one. An API key is accepted
// so a heavy user can be identified for fair-use accounting, never to unlock
// anything an anonymous caller cannot reach.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api-geo.digitalghana.dev/v1"

type Client struct {
	BaseURL string
	APIKey  string
	http    *http.Client
	version string
}

func New(baseURL, apiKey, version string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		http:    &http.Client{Timeout: 20 * time.Second},
		version: version,
	}
}

// APIError carries the stable machine code from the published error catalog,
// so a script can branch on `code` rather than parsing prose.
type APIError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details"`
	Docs      string         `json:"docs"`
	Status    int            `json:"-"`
}

func (e *APIError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("%s: %s (request %s)", e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	u := c.BaseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("ghanageo-cli/%s (%s; %s)", c.version, runtime.GOOS, runtime.GOARCH))
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach %s: %w", c.BaseURL, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		var wrapper struct {
			Error APIError `json:"error"`
		}
		if json.Unmarshal(body, &wrapper) == nil && wrapper.Error.Code != "" {
			wrapper.Error.Status = res.StatusCode
			return &wrapper.Error
		}
		return &APIError{
			Code:    "HTTP_" + strconv.Itoa(res.StatusCode),
			Message: strings.TrimSpace(string(body)),
			Status:  res.StatusCode,
		}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

// ---- Response shapes. Only the fields the CLI renders. ----

type Ref struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Region struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	Capital            string      `json:"capital"`
	Code               string      `json:"code"`
	VerificationStatus string      `json:"verificationStatus"`
	Centroid           *Coordinate `json:"centroid"`
}

type District struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	Code               string      `json:"code"`
	Type               string      `json:"type"`
	Capital            string      `json:"capital"`
	Region             Ref         `json:"region"`
	VerificationStatus string      `json:"verificationStatus"`
	Centroid           *Coordinate `json:"centroid"`
}

type Place struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	Type               string      `json:"type"`
	Region             *Ref        `json:"region"`
	District           *Ref        `json:"district"`
	VerificationStatus string      `json:"verificationStatus"`
	Centroid           *Coordinate `json:"centroid"`
}

type SearchResult struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Kind         string  `json:"kind"`
	RegionName   string  `json:"regionName"`
	DistrictName string  `json:"districtName"`
	Score        float64 `json:"score"`
	MatchReason  string  `json:"matchReason"`
}

type page[T any] struct {
	Data           []T    `json:"data"`
	NextCursor     string `json:"nextCursor"`
	DatasetVersion string `json:"datasetVersion"`
}

type Page[T any] struct {
	Data           []T
	NextCursor     string
	DatasetVersion string
}

func fetchPage[T any](ctx context.Context, c *Client, path string, p url.Values) (Page[T], error) {
	var raw page[T]
	if err := c.get(ctx, path, p, &raw); err != nil {
		return Page[T]{}, err
	}
	return Page[T]{Data: raw.Data, NextCursor: raw.NextCursor, DatasetVersion: raw.DatasetVersion}, nil
}

func (c *Client) Regions(ctx context.Context, limit int, cursor string) (Page[Region], error) {
	return fetchPage[Region](ctx, c, "/regions", listParams(limit, cursor))
}

func (c *Client) Districts(ctx context.Context, regionID, query string, limit int, cursor string) (Page[District], error) {
	p := listParams(limit, cursor)
	if regionID != "" {
		p.Set("regionId", regionID)
	}
	if query != "" {
		p.Set("q", query)
	}
	return fetchPage[District](ctx, c, "/districts", p)
}

func (c *Client) Places(ctx context.Context, districtID, regionID, placeType, query string, limit int, cursor string) (Page[Place], error) {
	p := listParams(limit, cursor)
	for k, v := range map[string]string{
		"districtId": districtID, "regionId": regionID, "type": placeType, "q": query,
	} {
		if v != "" {
			p.Set(k, v)
		}
	}
	return fetchPage[Place](ctx, c, "/places", p)
}

func (c *Client) Search(ctx context.Context, query, regionID, placeType string, limit int) (Page[SearchResult], error) {
	p := url.Values{}
	p.Set("q", query)
	if limit > 0 {
		p.Set("limit", strconv.Itoa(limit))
	}
	if regionID != "" {
		p.Set("regionId", regionID)
	}
	if placeType != "" {
		p.Set("type", placeType)
	}
	return fetchPage[SearchResult](ctx, c, "/search", p)
}

func (c *Client) Autocomplete(ctx context.Context, query string, limit int) (Page[SearchResult], error) {
	p := url.Values{}
	p.Set("q", query)
	if limit > 0 {
		p.Set("limit", strconv.Itoa(limit))
	}
	return fetchPage[SearchResult](ctx, c, "/autocomplete", p)
}

func (c *Client) Nearby(ctx context.Context, lat, lng float64, radius, limit int) (Page[Place], error) {
	p := url.Values{}
	p.Set("lat", formatFloat(lat))
	p.Set("lng", formatFloat(lng))
	if radius > 0 {
		p.Set("radius", strconv.Itoa(radius))
	}
	if limit > 0 {
		p.Set("limit", strconv.Itoa(limit))
	}
	return fetchPage[Place](ctx, c, "/nearby", p)
}

type ReverseResult struct {
	Region         *Ref    `json:"region"`
	District       *Ref    `json:"district"`
	Nearby         []Place `json:"nearby"`
	DatasetVersion string  `json:"datasetVersion"`
}

func (c *Client) Reverse(ctx context.Context, lat, lng float64) (ReverseResult, error) {
	p := url.Values{}
	p.Set("lat", formatFloat(lat))
	p.Set("lng", formatFloat(lng))
	var out ReverseResult
	err := c.get(ctx, "/reverse", p, &out)
	return out, err
}

func (c *Client) Health(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	// /health sits outside the versioned prefix.
	base := strings.TrimSuffix(c.BaseURL, "/v1")
	old := c.BaseURL
	c.BaseURL = base
	err := c.get(ctx, "/health", nil, &out)
	c.BaseURL = old
	return out, err
}

func listParams(limit int, cursor string) url.Values {
	p := url.Values{}
	if limit > 0 {
		p.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		p.Set("cursor", cursor)
	}
	return p
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
