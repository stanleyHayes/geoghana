// Package typesense implements ports.SearchPort against a Typesense server.
//
// Why Typesense and not MongoDB text search: self-hosted MongoDB's $text has
// no fuzzy matching or typo tolerance, and Spec section 10 requires both.
// Atlas Search would provide them but ties the product to a managed service.
// The SearchPort abstraction keeps both available; this implementation runs
// locally, in CI, and in any self-hosted deployment.
package typesense

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/normalize"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

const Collection = "ghanageo_places"

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		switch v := body.(type) {
		case string:
			rdr = strings.NewReader(v)
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return err
			}
			rdr = bytes.NewReader(b)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("X-TYPESENSE-API-KEY", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("typesense %s %s: %w", method, path, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("typesense %s %s: %d: %s", method, path, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// Rebuild drops and recreates the collection.
//
// Import is UPSERT-ONLY, so a reindex adds and updates but never removes a
// document whose source record has gone. After deduplication merged places
// stayed searchable and outranked their own survivors — the index quietly
// disagreed with the database.
//
// The tradeoff is a brief window with no search. That is acceptable while the
// dataset rebuilds in seconds; at production scale the answer is to build a
// new collection and swap an alias, so the old index serves until the new one
// is complete.
func (c *Client) Rebuild(ctx context.Context) error {
	// A missing collection is the desired end state, so a 404 is success.
	if err := c.do(ctx, http.MethodDelete, "/collections/"+Collection, nil, nil); err != nil {
		if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "Not Found") {
			return fmt.Errorf("drop collection: %w", err)
		}
	}
	return c.EnsureSchema(ctx)
}

// EnsureSchema creates the collection if it does not exist. Idempotent.
func (c *Client) EnsureSchema(ctx context.Context) error {
	schema := map[string]any{
		"name": Collection,
		"fields": []map[string]any{
			{"name": "id", "type": "string"},
			{"name": "name", "type": "string"},
			// The folded form: matching a typed "kwabenya" against "Kwabɛnya".
			{"name": "normalized", "type": "string"},
			{"name": "aliases", "type": "string[]", "optional": true},
			{"name": "type", "type": "string", "facet": true},
			{"name": "regionId", "type": "string", "facet": true, "optional": true},
			{"name": "regionName", "type": "string", "optional": true},
			{"name": "districtId", "type": "string", "facet": true, "optional": true},
			{"name": "districtName", "type": "string", "optional": true},
			{"name": "kind", "type": "string", "facet": true},
			// Rank exact administrative units above minor localities.
			{"name": "weight", "type": "int32"},
		},
		"default_sorting_field": "weight",
	}
	err := c.do(ctx, http.MethodPost, "/collections", schema, nil)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

type indexDoc struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Normalized   string   `json:"normalized"`
	Aliases      []string `json:"aliases,omitempty"`
	Type         string   `json:"type"`
	RegionID     string   `json:"regionId,omitempty"`
	RegionName   string   `json:"regionName,omitempty"`
	DistrictID   string   `json:"districtId,omitempty"`
	DistrictName string   `json:"districtName,omitempty"`
	Kind         string   `json:"kind"`
	Weight       int32    `json:"weight"`
}

// Index upserts documents in a single JSONL import.
func (c *Client) Index(ctx context.Context, docs []ports.SearchDoc) error {
	if len(docs) == 0 {
		return nil
	}
	var b strings.Builder
	for _, d := range docs {
		normalizedAliases := make([]string, 0, len(d.Aliases)*2)
		for _, a := range d.Aliases {
			normalizedAliases = append(normalizedAliases, a, normalize.Name(a))
		}
		line, err := json.Marshal(indexDoc{
			ID:           d.ID,
			Name:         d.Name,
			Normalized:   normalize.Name(d.Name),
			Aliases:      normalizedAliases,
			Type:         d.Type,
			RegionID:     d.RegionID,
			RegionName:   d.RegionName,
			DistrictID:   d.DistrictID,
			DistrictName: d.DistrictName,
			Kind:         d.Kind,
			Weight:       d.Weight,
		})
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	// The import endpoint answers with JSONL — one result object per document,
	// not a single JSON value — and reports per-document failures with HTTP 200.
	// Ignoring the body would silently drop records from the index.
	path := "/collections/" + Collection + "/documents/import?action=upsert"
	raw, err := c.doRaw(ctx, http.MethodPost, path, b.String())
	if err != nil {
		return err
	}
	var failures []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var r struct {
			Success  bool   `json:"success"`
			Error    string `json:"error"`
			Document string `json:"document"`
		}
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return fmt.Errorf("typesense import: unparseable result line: %w", err)
		}
		if !r.Success {
			failures = append(failures, r.Error)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("typesense import: %d of %d documents failed, first: %s",
			len(failures), len(docs), failures[0])
	}
	return nil
}

// doRaw performs a request and returns the body unparsed, for endpoints whose
// response is not a single JSON value.
func (c *Client) doRaw(ctx context.Context, method, path, body string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-TYPESENSE-API-KEY", c.apiKey)
	req.Header.Set("Content-Type", "text/plain")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("typesense %s %s: %w", method, path, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("typesense %s %s: %d: %s", method, path, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

type tsHit struct {
	Document  indexDoc `json:"document"`
	TextMatch int64    `json:"text_match"`
	// Typesense's `highlight` is deliberately NOT parsed. Its shape varies by
	// field type — an object for a string field, an ARRAY for a string[] field
	// like aliases — so decoding it into one struct crashed on any multi-token
	// query that matched an alias, which included the "tema comm 25" case.
	//
	// Nothing is lost: the match explanation callers receive is computed in
	// internal/domain/relevance, so both SearchPort implementations produce
	// identical reasons. Parsing this only created a way to fail.
}

type tsResponse struct {
	Found int     `json:"found"`
	Hits  []tsHit `json:"hits"`
}

func (c *Client) Search(ctx context.Context, q ports.SearchQuery) ([]ports.SearchHit, error) {
	// Search the folded form too, so a query typed in ASCII finds a name
	// written with Twi, Ga or Ewe characters.
	params := url.Values{}
	params.Set("q", q.Text)
	params.Set("query_by", "name,normalized,aliases")
	params.Set("query_by_weights", "4,3,2")
	params.Set("num_typos", "2")
	params.Set("prefix", "true,true,true")
	params.Set("per_page", strconv.Itoa(clampLimit(q.Limit)))
	params.Set("sort_by", "_text_match:desc,weight:desc")

	var filters []string
	if q.RegionID != "" {
		filters = append(filters, "regionId:="+q.RegionID)
	}
	if q.DistrictID != "" {
		filters = append(filters, "districtId:="+q.DistrictID)
	}
	if q.Type != "" {
		filters = append(filters, "type:="+q.Type)
	}
	if len(filters) > 0 {
		params.Set("filter_by", strings.Join(filters, " && "))
	}
	return c.runSearch(ctx, params)
}

// Autocomplete favours prefix matching and tolerates fewer typos, because a
// partial word must not be "corrected" into a different place.
func (c *Client) Autocomplete(ctx context.Context, prefix string, limit int) ([]ports.SearchHit, error) {
	params := url.Values{}
	params.Set("q", prefix)
	params.Set("query_by", "name,normalized,aliases")
	params.Set("query_by_weights", "4,3,2")
	params.Set("num_typos", "1")
	params.Set("prefix", "true,true,true")
	params.Set("per_page", strconv.Itoa(clampLimit(limit)))
	params.Set("sort_by", "_text_match:desc,weight:desc")
	return c.runSearch(ctx, params)
}

func (c *Client) runSearch(ctx context.Context, params url.Values) ([]ports.SearchHit, error) {
	var res tsResponse
	path := "/collections/" + Collection + "/documents/search?" + params.Encode()
	if err := c.do(ctx, http.MethodGet, path, nil, &res); err != nil {
		return nil, err
	}

	out := make([]ports.SearchHit, 0, len(res.Hits))
	best := int64(0)
	for _, h := range res.Hits {
		if h.TextMatch > best {
			best = h.TextMatch
		}
	}
	for _, h := range res.Hits {
		// Normalise Typesense's absolute text_match into a 0..1 confidence,
		// which is what Spec section 10 requires callers to receive.
		score := 1.0
		if best > 0 {
			score = float64(h.TextMatch) / float64(best)
		}
		out = append(out, ports.SearchHit{
			Doc: ports.SearchDoc{
				ID:           h.Document.ID,
				Name:         h.Document.Name,
				Normalized:   h.Document.Normalized,
				Type:         h.Document.Type,
				RegionID:     h.Document.RegionID,
				RegionName:   h.Document.RegionName,
				DistrictID:   h.Document.DistrictID,
				DistrictName: h.Document.DistrictName,
				Kind:         h.Document.Kind,
				Weight:       h.Document.Weight,
			},
			Score: round2(score),
			// Overwritten by the domain scorer in app/search.rescore.
			MatchReason: "",
		})
	}
	return out, nil
}

func clampLimit(n int) int {
	if n <= 0 {
		return 20
	}
	if n > 100 {
		return 100
	}
	return n
}

func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
