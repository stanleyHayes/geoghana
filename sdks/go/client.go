package ghanageo

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxRetries = 5
const maxRetryDelay = 60 * time.Second
const DefaultMaxDownloadBytes int64 = 64 << 20

type TelemetryEvent struct {
	Path, Method    string
	Attempt, Status int
	Duration        time.Duration
	ErrorCode       string
}
type TelemetryObserver func(TelemetryEvent)
type DownloadOptions struct {
	MaxBytes int64
	SHA256   string
}

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}
type Option func(*Client) error

type Client struct {
	baseURL   string
	apiKey    string
	userAgent string
	http      HTTPDoer
	retries   int
	baseDelay time.Duration
	maxDelay  time.Duration
	observer  TelemetryObserver
	sleep     func(context.Context, time.Duration) error
}

func New(options ...Option) (*Client, error) {
	c := &Client{baseURL: DefaultBaseURL, userAgent: "ghanageo-go/" + SDKVersion, http: http.DefaultClient, retries: 2, baseDelay: 100 * time.Millisecond, maxDelay: 2 * time.Second, sleep: sleepContext}
	for _, option := range options {
		if err := option(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func WithBaseURL(value string) Option {
	return func(c *Client) error {
		u, err := url.Parse(strings.TrimRight(value, "/"))
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("ghanageo: invalid base URL %q", value)
		}
		c.baseURL = u.String()
		return nil
	}
}
func WithAPIKey(value string) Option { return func(c *Client) error { c.apiKey = value; return nil } }
func WithUserAgent(value string) Option {
	return func(c *Client) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("ghanageo: user agent cannot be empty")
		}
		c.userAgent = value
		return nil
	}
}
func WithHTTPClient(value HTTPDoer) Option {
	return func(c *Client) error {
		if value == nil {
			return fmt.Errorf("ghanageo: HTTP client cannot be nil")
		}
		c.http = value
		return nil
	}
}

// WithTelemetry installs a local metadata-only observer. No telemetry is emitted by default.
func WithTelemetry(observer TelemetryObserver) Option {
	return func(c *Client) error { c.observer = observer; return nil }
}
func WithRetry(retries int, baseDelay, maximumDelay time.Duration) Option {
	return func(c *Client) error {
		if retries < 0 {
			return fmt.Errorf("ghanageo: retries cannot be negative")
		}
		if baseDelay < 0 || maximumDelay < 0 {
			return fmt.Errorf("ghanageo: retry delays cannot be negative")
		}
		if retries > maxRetries {
			retries = maxRetries
		}
		if maximumDelay > maxRetryDelay {
			maximumDelay = maxRetryDelay
		}
		if baseDelay > maximumDelay {
			baseDelay = maximumDelay
		}
		c.retries, c.baseDelay, c.maxDelay = retries, baseDelay, maximumDelay
		return nil
	}
}
func withSleep(fn func(context.Context, time.Duration) error) Option {
	return func(c *Client) error { c.sleep = fn; return nil }
}

func (c *Client) APIVersion() string           { return APIVersion }
func (c *Client) TestedDatasetVersion() string { return TestedDatasetVersion }
func (c *Client) emit(event TelemetryEvent) {
	if c.observer == nil {
		return
	}
	defer func() { _ = recover() }()
	c.observer(event)
}

// Request performs a typed low-level safe read for advanced endpoints and conformance tooling.
func (c *Client) Request(ctx context.Context, path string, query url.Values, out any) error {
	return c.request(ctx, path, query, out)
}

func (c *Client) request(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	for attempt := 0; attempt <= c.retries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.userAgent)
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		started := time.Now()
		resp, err := c.http.Do(req)
		if err != nil {
			c.emit(TelemetryEvent{Path: path, Method: http.MethodGet, Attempt: attempt, Duration: time.Since(started)})
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if attempt == c.retries {
				return err
			}
			if err = c.sleep(ctx, c.retryDelay(attempt, "")); err != nil {
				return err
			}
			continue
		}
		if isRetryable(resp.StatusCode) && attempt < c.retries {
			c.emit(TelemetryEvent{Path: path, Method: http.MethodGet, Attempt: attempt, Status: resp.StatusCode, Duration: time.Since(started)})
			io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if err = c.sleep(ctx, c.retryDelay(attempt, resp.Header.Get("Retry-After"))); err != nil {
				return err
			}
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			failure := decodeError(resp)
			var apiErr *Error
			_ = errors.As(failure, &apiErr)
			code := ""
			if apiErr != nil {
				code = apiErr.Code
			}
			c.emit(TelemetryEvent{Path: path, Method: http.MethodGet, Attempt: attempt, Status: resp.StatusCode, Duration: time.Since(started), ErrorCode: code})
			return failure
		}
		if err = json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out); err != nil {
			c.emit(TelemetryEvent{Path: path, Method: http.MethodGet, Attempt: attempt, Status: resp.StatusCode, Duration: time.Since(started), ErrorCode: "DECODE_ERROR"})
			return fmt.Errorf("ghanageo: decode response: %w", err)
		}
		c.emit(TelemetryEvent{Path: path, Method: http.MethodGet, Attempt: attempt, Status: resp.StatusCode, Duration: time.Since(started)})
		return nil
	}
	return fmt.Errorf("ghanageo: retry loop exhausted")
}

func isRetryable(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}
func (c *Client) retryDelay(attempt int, retryAfter string) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds >= 0 {
		d := time.Duration(seconds) * time.Second
		if d > maxRetryDelay {
			return maxRetryDelay
		}
		if d > c.maxDelay {
			return c.maxDelay
		}
		return d
	}
	if when, err := http.ParseTime(retryAfter); err == nil {
		d := time.Until(when)
		if d < 0 {
			return 0
		}
		if d > c.maxDelay {
			return c.maxDelay
		}
		return d
	}
	d := c.baseDelay * time.Duration(1<<attempt)
	if d > c.maxDelay {
		return c.maxDelay
	}
	return d
}
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func decodeError(resp *http.Response) error {
	var envelope struct {
		Error Error `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&envelope); err != nil {
		return &Error{Status: resp.StatusCode, Code: "HTTP_ERROR", Message: http.StatusText(resp.StatusCode), RequestID: resp.Header.Get("X-Request-ID")}
	}
	envelope.Error.Status = resp.StatusCode
	if envelope.Error.Code == "" {
		envelope.Error.Code = "HTTP_ERROR"
	}
	if envelope.Error.RequestID == "" {
		envelope.Error.RequestID = resp.Header.Get("X-Request-ID")
	}
	if envelope.Error.Message == "" {
		envelope.Error.Message = http.StatusText(resp.StatusCode)
	}
	return &envelope.Error
}

func pageValues(p PageOptions) url.Values {
	q := url.Values{}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	if p.Limit != 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	return q
}
func set(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}
func (c *Client) Regions(ctx context.Context, o PageOptions) (Page[Region], error) {
	var v Page[Region]
	err := c.request(ctx, "/regions", pageValues(o), &v)
	return v, err
}
func (c *Client) Region(ctx context.Context, id string) (Region, error) {
	var v Region
	err := c.request(ctx, "/regions/"+url.PathEscape(id), nil, &v)
	return v, err
}
func (c *Client) RegionDistricts(ctx context.Context, id string, o PageOptions) (Page[District], error) {
	var v Page[District]
	err := c.request(ctx, "/regions/"+url.PathEscape(id)+"/districts", pageValues(o), &v)
	return v, err
}
func (c *Client) Districts(ctx context.Context, o DistrictOptions) (Page[District], error) {
	q := pageValues(o.PageOptions)
	set(q, "regionId", o.RegionID)
	set(q, "q", o.Query)
	var v Page[District]
	err := c.request(ctx, "/districts", q, &v)
	return v, err
}
func (c *Client) District(ctx context.Context, id string) (District, error) {
	var v District
	err := c.request(ctx, "/districts/"+url.PathEscape(id), nil, &v)
	return v, err
}
func (c *Client) DistrictPlaces(ctx context.Context, id string, o PlaceOptions) (Page[Place], error) {
	q := pageValues(o.PageOptions)
	set(q, "type", o.Type)
	var v Page[Place]
	err := c.request(ctx, "/districts/"+url.PathEscape(id)+"/places", q, &v)
	return v, err
}
func (c *Client) Places(ctx context.Context, o PlaceOptions) (Page[Place], error) {
	q := pageValues(o.PageOptions)
	set(q, "districtId", o.DistrictID)
	set(q, "regionId", o.RegionID)
	set(q, "type", o.Type)
	set(q, "q", o.Query)
	var v Page[Place]
	err := c.request(ctx, "/places", q, &v)
	return v, err
}
func (c *Client) Place(ctx context.Context, id string) (Place, error) {
	var v Place
	err := c.request(ctx, "/places/"+url.PathEscape(id), nil, &v)
	return v, err
}
func (c *Client) Search(ctx context.Context, query string, o SearchOptions) (Page[SearchResult], error) {
	q := url.Values{"q": {query}}
	set(q, "regionId", o.RegionID)
	set(q, "districtId", o.DistrictID)
	set(q, "type", o.Type)
	if o.Limit != 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	var v Page[SearchResult]
	err := c.request(ctx, "/search", q, &v)
	return v, err
}
func (c *Client) Autocomplete(ctx context.Context, query string, limit int) (Page[SearchResult], error) {
	return c.searchLike(ctx, "/autocomplete", query, limit)
}
func (c *Client) Geocode(ctx context.Context, query string, limit int) (Page[SearchResult], error) {
	return c.searchLike(ctx, "/geocode", query, limit)
}
func (c *Client) searchLike(ctx context.Context, path, query string, limit int) (Page[SearchResult], error) {
	q := url.Values{"q": {query}}
	if limit != 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var v Page[SearchResult]
	err := c.request(ctx, path, q, &v)
	return v, err
}
func (c *Client) ReverseGeocode(ctx context.Context, lat, lng float64) (ReverseResult, error) {
	q := url.Values{"lat": {strconv.FormatFloat(lat, 'f', -1, 64)}, "lng": {strconv.FormatFloat(lng, 'f', -1, 64)}}
	var v ReverseResult
	err := c.request(ctx, "/reverse", q, &v)
	return v, err
}
func (c *Client) Nearby(ctx context.Context, lat, lng float64, radius, limit int) (Page[Place], error) {
	q := url.Values{"lat": {strconv.FormatFloat(lat, 'f', -1, 64)}, "lng": {strconv.FormatFloat(lng, 'f', -1, 64)}}
	if radius != 0 {
		q.Set("radius", strconv.Itoa(radius))
	}
	if limit != 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var v Page[Place]
	err := c.request(ctx, "/nearby", q, &v)
	return v, err
}
func (c *Client) Boundary(ctx context.Context, id string) (BoundaryFeature, error) {
	var v BoundaryFeature
	err := c.request(ctx, "/boundaries/"+url.PathEscape(id), nil, &v)
	return v, err
}
func (c *Client) Datasets(ctx context.Context) (DatasetPage, error) {
	var v DatasetPage
	err := c.request(ctx, "/datasets", nil, &v)
	return v, err
}
func (c *Client) DatasetDownloads(ctx context.Context, version string) (DownloadList, error) {
	var v DownloadList
	err := c.request(ctx, "/datasets/"+url.PathEscape(version)+"/downloads", nil, &v)
	return v, err
}
func (c *Client) Roads(ctx context.Context) (map[string]any, error) {
	var v map[string]any
	err := c.request(ctx, "/roads", nil, &v)
	return v, err
}
func (c *Client) PointsOfInterest(ctx context.Context) (map[string]any, error) {
	var v map[string]any
	err := c.request(ctx, "/pois", nil, &v)
	return v, err
}

func (c *Client) DownloadDatasetArtifact(ctx context.Context, version, entity, format string) ([]byte, error) {
	var buffer strings.Builder
	if err := c.download(ctx, version, entity, format, &buffer, DownloadOptions{}); err != nil {
		return nil, err
	}
	return []byte(buffer.String()), nil
}

// DownloadDatasetArtifactTo streams into a temporary sibling and atomically replaces destination
// only after the size limit and optional lowercase/uppercase SHA-256 digest are verified.
func (c *Client) DownloadDatasetArtifactTo(ctx context.Context, version, entity, format, destination string, options DownloadOptions) error {
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".ghanageo-download-*")
	if err != nil {
		return err
	}
	name, committed := temporary.Name(), false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(name)
		}
	}()
	if err = c.download(ctx, version, entity, format, temporary, options); err != nil {
		return err
	}
	if err = temporary.Sync(); err != nil {
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, destination); err != nil {
		return err
	}
	committed = true
	return nil
}

func (c *Client) download(ctx context.Context, version, entity, format string, destination io.Writer, options DownloadOptions) error {
	limit := options.MaxBytes
	if limit <= 0 {
		limit = DefaultMaxDownloadBytes
	}
	path := "/datasets/" + url.PathEscape(version) + "/downloads/" + url.PathEscape(entity) + "." + url.PathEscape(format)
	for attempt := 0; attempt <= c.retries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/octet-stream")
		req.Header.Set("User-Agent", c.userAgent)
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		started := time.Now()
		resp, err := c.http.Do(req)
		if err != nil {
			c.emit(TelemetryEvent{Path: path, Method: http.MethodGet, Attempt: attempt, Duration: time.Since(started)})
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if attempt == c.retries {
				return err
			}
			if err = c.sleep(ctx, c.retryDelay(attempt, "")); err != nil {
				return err
			}
			continue
		}
		if isRetryable(resp.StatusCode) && attempt < c.retries {
			c.emit(TelemetryEvent{Path: path, Method: http.MethodGet, Attempt: attempt, Status: resp.StatusCode, Duration: time.Since(started)})
			io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if err = c.sleep(ctx, c.retryDelay(attempt, resp.Header.Get("Retry-After"))); err != nil {
				return err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			defer resp.Body.Close()
			failure := decodeError(resp)
			var apiErr *Error
			_ = errors.As(failure, &apiErr)
			code := ""
			if apiErr != nil {
				code = apiErr.Code
			}
			c.emit(TelemetryEvent{Path: path, Method: http.MethodGet, Attempt: attempt, Status: resp.StatusCode, Duration: time.Since(started), ErrorCode: code})
			return failure
		}
		defer resp.Body.Close()
		hash := sha256.New()
		written, err := io.Copy(io.MultiWriter(destination, hash), io.LimitReader(resp.Body, limit+1))
		if err != nil {
			c.emit(TelemetryEvent{Path: path, Method: http.MethodGet, Attempt: attempt, Status: resp.StatusCode, Duration: time.Since(started), ErrorCode: "DOWNLOAD_READ_ERROR"})
			return err
		}
		if written > limit {
			c.emit(TelemetryEvent{Path: path, Method: http.MethodGet, Attempt: attempt, Status: resp.StatusCode, Duration: time.Since(started), ErrorCode: ErrDownloadTooLarge.Code})
			return ErrDownloadTooLarge
		}
		if options.SHA256 != "" && !strings.EqualFold(options.SHA256, fmt.Sprintf("%x", hash.Sum(nil))) {
			c.emit(TelemetryEvent{Path: path, Method: http.MethodGet, Attempt: attempt, Status: resp.StatusCode, Duration: time.Since(started), ErrorCode: ErrChecksumMismatch.Code})
			return ErrChecksumMismatch
		}
		c.emit(TelemetryEvent{Path: path, Method: http.MethodGet, Attempt: attempt, Status: resp.StatusCode, Duration: time.Since(started)})
		return nil
	}
	return fmt.Errorf("ghanageo: retry loop exhausted")
}
