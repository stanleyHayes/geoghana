// Command conformance exercises the public Go REST facade against the shared live fixture.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	ghanageo "github.com/ghanageo/ghanageo-go"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type contractCase struct {
	ID        string         `json:"id"`
	Operation string         `json:"operation"`
	Protocols []string       `json:"protocols"`
	Input     map[string]any `json:"input"`
	Expect    struct {
		Outcome    string `json:"outcome"`
		HTTPStatus int    `json:"httpStatus"`
		Shape      struct {
			Required   []string          `json:"required"`
			FieldTypes map[string]string `json:"fieldTypes"`
		} `json:"shape"`
		Error struct {
			Code           string   `json:"code"`
			RequiredFields []string `json:"requiredFields"`
		} `json:"error"`
		QuotaCost map[string]any `json:"quotaCost"`
		Semantics []string       `json:"semantics"`
	} `json:"expect"`
}
type exported struct {
	ContractDigest string         `json:"contractDigest"`
	Cases          []contractCase `json:"cases"`
}
type result struct {
	CaseID    string   `json:"caseId"`
	Status    string   `json:"status"`
	Protocols []string `json:"protocols"`
	Message   string   `json:"message,omitempty"`
}
type evidence struct {
	CaseID         string `json:"caseId"`
	Protocol       string `json:"protocol"`
	RequestDigest  string `json:"requestDigest"`
	ResponseDigest string `json:"responseDigest"`
}
type caseTransport struct {
	id            string
	status        int
	authorization bool
	quota         map[string]any
	requests      []requestTrace
}
type requestTrace struct {
	Method        string `json:"method"`
	Path          string `json:"path"`
	Query         string `json:"query"`
	Authorization bool   `json:"authorizationSent"`
	BodyDigest    string `json:"bodyDigest"`
}

func (t *caseTransport) Do(r *http.Request) (*http.Response, error) {
	body := []byte{}
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
	}
	t.requests = append(t.requests, requestTrace{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("Authorization") != "", digest(body)})
	r.Header.Set("X-Conformance-Case", t.id)
	t.authorization = r.Header.Get("Authorization") != ""
	response, err := http.DefaultClient.Do(r)
	if response != nil {
		t.status = response.StatusCode
		_ = json.Unmarshal([]byte(response.Header.Get("X-Conformance-Quota")), &t.quota)
	}
	return response, err
}
func digest(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func has(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func object(input map[string]any, key string) map[string]any {
	v, _ := input[key].(map[string]any)
	return v
}
func str(v any) string     { s, _ := v.(string); return s }
func integer(v any) int    { n, _ := v.(float64); return int(n) }
func number(v any) float64 { n, _ := v.(float64); return n }
func call(ctx context.Context, c *ghanageo.Client, x contractCase) (any, error) {
	p, q := object(x.Input, "path"), object(x.Input, "query")
	if x.ID == "error.invalid-argument" {
		var out map[string]any
		err := c.Request(ctx, "/places", url.Values{"limit": {"invalid"}}, &out)
		return out, err
	}
	page := ghanageo.PageOptions{Cursor: str(q["cursor"]), Limit: integer(q["limit"])}
	switch x.Operation {
	case "listRegions":
		return c.Regions(ctx, page)
	case "getRegion":
		return c.Region(ctx, str(p["id"]))
	case "listRegionDistricts":
		return c.RegionDistricts(ctx, str(p["id"]), page)
	case "listDistricts":
		return c.Districts(ctx, ghanageo.DistrictOptions{PageOptions: page, RegionID: str(q["regionId"]), Query: str(q["q"])})
	case "getDistrict":
		return c.District(ctx, str(p["id"]))
	case "listDistrictPlaces":
		return c.DistrictPlaces(ctx, str(p["id"]), ghanageo.PlaceOptions{PageOptions: page, Type: str(q["type"])})
	case "listPlaces":
		return c.Places(ctx, ghanageo.PlaceOptions{PageOptions: page, RegionID: str(q["regionId"]), DistrictID: str(q["districtId"]), Type: str(q["type"]), Query: str(q["q"])})
	case "getPlace":
		return c.Place(ctx, str(p["id"]))
	case "search":
		return c.Search(ctx, str(q["q"]), ghanageo.SearchOptions{RegionID: str(q["regionId"]), DistrictID: str(q["districtId"]), Type: str(q["type"]), Limit: integer(q["limit"])})
	case "autocomplete":
		return c.Autocomplete(ctx, str(q["q"]), integer(q["limit"]))
	case "geocode":
		return c.Geocode(ctx, str(q["q"]), integer(q["limit"]))
	case "reverseGeocode":
		return c.ReverseGeocode(ctx, number(q["lat"]), number(q["lng"]))
	case "nearby":
		return c.Nearby(ctx, number(q["lat"]), number(q["lng"]), integer(q["radius"]), integer(q["limit"]))
	case "getBoundary":
		return c.Boundary(ctx, str(p["id"]))
	case "listDatasets":
		return c.Datasets(ctx)
	case "listDatasetDownloads":
		return c.DatasetDownloads(ctx, str(p["version"]))
	case "downloadDatasetArtifact":
		return c.DownloadDatasetArtifact(ctx, str(p["version"]), str(p["entity"]), str(p["format"]))
	case "listRoads":
		return c.Roads(ctx)
	case "listPointsOfInterest":
		return c.PointsOfInterest(ctx)
	default:
		return nil, fmt.Errorf("no Go REST mapping for %s", x.Operation)
	}
}
func inputFailures(c contractCase, requests []requestTrace) []string {
	fail := []string{}
	if len(requests) == 0 {
		return []string{"no HTTP request captured"}
	}
	actual, _ := url.ParseQuery(requests[0].Query)
	for key, value := range object(c.Input, "query") {
		want := fmt.Sprint(value)
		if actual.Get(key) != want {
			fail = append(fail, fmt.Sprintf("query %s mapped as %q, want %q", key, actual.Get(key), want))
		}
	}
	for key, value := range object(c.Input, "path") {
		if !strings.Contains(requests[0].Path, url.PathEscape(fmt.Sprint(value))) {
			fail = append(fail, fmt.Sprintf("path %s was not mapped", key))
		}
	}
	return fail
}
func normalized(v any) any {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}
func datasetVersions(v any, versions map[string]bool) {
	switch node := v.(type) {
	case map[string]any:
		for key, child := range node {
			if key == "datasetVersion" {
				if value, ok := child.(string); ok && value != "" {
					versions[value] = true
				}
			}
			datasetVersions(child, versions)
		}
	case []any:
		for _, child := range node {
			datasetVersions(child, versions)
		}
	}
}
func pathValue(v any, path string) (any, bool) {
	current := v
	for _, part := range strings.Split(path, ".") {
		node, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = node[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
func kind(v any) string {
	switch v.(type) {
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	}
	return "unknown"
}
func expectedKind(path string) string {
	leaf := path[strings.LastIndex(path, ".")+1:]
	if has([]string{"data", "aliases", "downloads", "nearby"}, leaf) {
		return "array"
	}
	if has([]string{"error", "provenance", "region", "geometry", "properties", "change", "details"}, leaf) {
		return "object"
	}
	if has([]string{"latitude", "longitude", "maxRadiusMeters", "minLength", "retryAfterSeconds", "limit", "cost", "maxCost", "depth", "maxDepth"}, leaf) {
		return "number"
	}
	return "string"
}
func validate(c contractCase, value any, callErr error, t *caseTransport) ([]string, any) {
	fail := []string{}
	outcome := "success"
	response := normalized(value)
	var apiErr *ghanageo.Error
	if callErr != nil {
		outcome = "error"
		if errors.As(callErr, &apiErr) {
			response = map[string]any{"error": normalized(apiErr)}
		} else {
			response = map[string]any{"error": callErr.Error()}
		}
	}
	if outcome != c.Expect.Outcome {
		fail = append(fail, fmt.Sprintf("expected %s, received %s", c.Expect.Outcome, outcome))
	}
	if c.Expect.HTTPStatus != 0 && t.status != c.Expect.HTTPStatus {
		fail = append(fail, fmt.Sprintf("expected HTTP %d, received %d", c.Expect.HTTPStatus, t.status))
	}
	if !reflect.DeepEqual(t.quota, c.Expect.QuotaCost) {
		fail = append(fail, "quota cost differs")
	}
	for _, path := range c.Expect.Shape.Required {
		got, ok := pathValue(response, path)
		if !ok {
			fail = append(fail, "missing result field "+path)
		} else if kind(got) != expectedKind(path) {
			fail = append(fail, fmt.Sprintf("%s must be %s", path, expectedKind(path)))
		}
	}
	for path, want := range c.Expect.Shape.FieldTypes {
		got, ok := pathValue(response, path)
		if !ok || kind(got) != want {
			fail = append(fail, fmt.Sprintf("%s must be %s", path, want))
		}
	}
	if c.Expect.Outcome == "error" {
		if apiErr == nil || apiErr.Code != c.Expect.Error.Code {
			fail = append(fail, "typed catalog error mismatch")
		}
		for _, field := range c.Expect.Error.RequiredFields {
			if _, ok := pathValue(response, "error."+field); !ok {
				fail = append(fail, "missing error field "+field)
			}
		}
	}
	if has(c.Expect.Semantics, "datasetVersionPresent") {
		v, ok := pathValue(response, "datasetVersion")
		if !ok || str(v) == "" {
			fail = append(fail, "datasetVersion absent")
		}
	}
	if has(c.Expect.Semantics, "anonymousByDefault") && t.authorization {
		fail = append(fail, "anonymous request sent authorization")
	}
	if has(c.Expect.Semantics, "preservesGhanaianOrthography") {
		b, _ := json.Marshal(response)
		if !strings.Contains(string(b), "Mampɔŋ") {
			fail = append(fail, "Ghanaian orthography not preserved")
		}
	}
	if has(c.Expect.Semantics, "emptyOutsideGhana") {
		node, _ := response.(map[string]any)
		nearby, _ := node["nearby"].([]any)
		if node["region"] != nil || node["district"] != nil || len(nearby) != 0 {
			fail = append(fail, "outside-Ghana result was not empty")
		}
	}
	versions := map[string]bool{}
	datasetVersions(response, versions)
	for version := range versions {
		if version != ghanageo.TestedDatasetVersion {
			fail = append(fail, fmt.Sprintf("observed dataset %s, SDK tested dataset is %s", version, ghanageo.TestedDatasetVersion))
		}
	}
	return fail, response
}
func main() {
	base := flag.String("base-url", "", "shared fixture base URL ending in /v1")
	output := flag.String("output", "go-conformance-report.json", "report path")
	root := flag.String("root", "../..", "repository root")
	flag.Parse()
	if *base == "" {
		fmt.Fprintln(os.Stderr, "--base-url is required")
		os.Exit(2)
	}
	absolute, err := filepath.Abs(*root)
	if err != nil {
		panic(err)
	}
	raw, err := exec.Command("ruby", filepath.Join(absolute, "tools/conformance/export_cases.rb")).Output()
	if err != nil {
		panic(err)
	}
	var contract exported
	if err = json.Unmarshal(raw, &contract); err != nil {
		panic(err)
	}
	transport := &caseTransport{}
	results := []result{}
	proof := []evidence{}
	passed := 0
	for _, c := range contract.Cases {
		if !has(c.Protocols, "rest") {
			continue
		}
		transport.id, transport.status, transport.authorization, transport.quota, transport.requests = c.ID, 0, false, nil, nil
		options := []ghanageo.Option{ghanageo.WithBaseURL(*base), ghanageo.WithHTTPClient(transport), ghanageo.WithRetry(0, 0, 0)}
		if auth := str(c.Input["auth"]); auth != "" && auth != "omitted" {
			options = append(options, ghanageo.WithAPIKey(auth))
		}
		client, clientErr := ghanageo.New(options...)
		if clientErr != nil {
			panic(clientErr)
		}
		ctx := context.Background()
		if has(c.Expect.Semantics, "cancellationPropagates") {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			go func() { time.Sleep(10 * time.Millisecond); cancel() }()
		}
		value, callErr := call(ctx, client, c)
		var fail []string
		var response any
		if has(c.Expect.Semantics, "cancellationPropagates") {
			response = map[string]any{"cancelled": errors.Is(callErr, context.Canceled)}
			if !errors.Is(callErr, context.Canceled) {
				fail = append(fail, "context cancellation did not propagate")
			}
		} else {
			fail, response = validate(c, value, callErr, transport)
		}
		fail = append(fail, inputFailures(c, transport.requests)...)
		if has(c.Expect.Semantics, "stableCursorPagination") {
			transport.id = c.ID
			it := client.PlacePages(ghanageo.PlaceOptions{PageOptions: ghanageo.PageOptions{Limit: 2}})
			ids := [][]string{}
			cursors := []string{}
			for it.Next(context.Background()) {
				page := it.Page()
				row := []string{}
				for _, place := range page.Data {
					row = append(row, place.ID)
				}
				ids = append(ids, row)
				cursors = append(cursors, page.NextCursor)
			}
			replayStable := false
			if len(cursors) > 0 {
				first, _ := client.Places(context.Background(), ghanageo.PlaceOptions{PageOptions: ghanageo.PageOptions{Cursor: cursors[0], Limit: 2}})
				second, _ := client.Places(context.Background(), ghanageo.PlaceOptions{PageOptions: ghanageo.PageOptions{Cursor: cursors[0], Limit: 2}})
				replayStable = reflect.DeepEqual(first.Data, second.Data)
			}
			if it.Err() != nil || len(ids) != 2 || len(cursors) < 1 || len(cursors[0]) < 8 || onlyDigits(cursors[0]) || overlap(ids[0], ids[1]) || !replayStable {
				fail = append(fail, "two-page cursor stability evidence invalid")
			}
		}
		status, message := "passed", ""
		if len(fail) > 0 {
			status, message = "failed", strings.Join(fail, "; ")
		} else {
			passed++
		}
		results = append(results, result{c.ID, status, []string{"rest"}, message})
		proof = append(proof, evidence{c.ID, "rest", digest(transport.requests), digest(map[string]any{"facade": response, "requests": transport.requests})})
	}
	report := map[string]any{"schemaVersion": 2, "contract": map[string]any{"digest": contract.ContractDigest, "runnerVersion": "1.0.0"}, "sdk": map[string]any{"language": "go", "name": "github.com/ghanageo/ghanageo-go", "version": ghanageo.SDKVersion, "supportedProtocols": []string{"rest"}}, "apiVersion": ghanageo.APIVersion, "datasetVersion": ghanageo.TestedDatasetVersion, "summary": map[string]int{"passed": passed, "failed": len(results) - passed, "skipped": 0}, "evidenceDigest": digest(proof), "evidence": proof, "results": results}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(*output, append(content, '\n'), 0o644); err != nil {
		panic(err)
	}
	if passed != len(results) {
		os.Exit(1)
	}
}
func onlyDigits(v string) bool {
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func overlap(a, b []string) bool {
	seen := map[string]bool{}
	for _, v := range a {
		seen[v] = true
	}
	for _, v := range b {
		if seen[v] {
			return true
		}
	}
	return false
}
