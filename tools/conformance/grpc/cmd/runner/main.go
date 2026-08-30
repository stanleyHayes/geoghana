package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	ghanageo "github.com/ghanageo/ghanageo-go"
	pb "github.com/ghanageo/ghanageo-go/proto/ghanageo/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const dataset = "2026.08.3-ulid"

type expected struct {
	Outcome   string
	Error     struct{ Code, GrpcStatus string }
	Semantics []string
}
type contractCase struct {
	ID, Operation string
	Protocols     []string
	Input         map[string]any
	Expect        expected
}
type corpus struct {
	ContractDigest string
	Cases          []contractCase
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

func main() {
	address := flag.String("address", "", "fixture host:port")
	casesPath := flag.String("cases", "", "exported cases")
	output := flag.String("output", "grpc-report.json", "report")
	flag.Parse()
	raw, err := os.ReadFile(*casesPath)
	if err != nil {
		panic(err)
	}
	var all corpus
	if err = json.Unmarshal(raw, &all); err != nil {
		panic(err)
	}
	conn, err := grpc.NewClient(*address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	client := pb.NewGeographyServiceClient(conn)
	results := []result{}
	proof := []evidence{}
	passed := 0
	for _, c := range all.Cases {
		if !has(c.Protocols, "grpc") {
			continue
		}
		value, callErr, request := call(client, c)
		fail := validate(c, value, callErr)
		if has(c.Expect.Semantics, "stableCursorPagination") {
			fail = append(fail, cursorProof(client, c)...)
		}
		statusValue, message := "passed", ""
		if len(fail) > 0 {
			statusValue = "failed"
			message = strings.Join(fail, "; ")
		} else {
			passed++
		}
		results = append(results, result{c.ID, statusValue, []string{"grpc"}, message})
		proof = append(proof, evidence{c.ID, "grpc", digest(request), digest(value)})
	}
	report := map[string]any{"schemaVersion": 2, "contract": map[string]any{"digest": all.ContractDigest, "runnerVersion": "1.0.0"}, "sdk": map[string]any{"language": "go", "name": "github.com/ghanageo/ghanageo-go/proto", "version": ghanageo.SDKVersion, "supportedProtocols": []string{"grpc"}}, "apiVersion": ghanageo.APIVersion, "datasetVersion": dataset, "summary": map[string]int{"passed": passed, "failed": len(results) - passed, "skipped": 0}, "evidenceDigest": digest(proof), "evidence": proof, "results": results}
	content, _ := json.MarshalIndent(report, "", "  ")
	if err = os.WriteFile(*output, append(content, '\n'), 0644); err != nil {
		panic(err)
	}
	if passed != len(results) {
		os.Exit(1)
	}
}
func has(v []string, w string) bool {
	for _, x := range v {
		if x == w {
			return true
		}
	}
	return false
}
func obj(m map[string]any, k string) map[string]any { v, _ := m[k].(map[string]any); return v }
func str(v any) string                              { s, _ := v.(string); return s }
func num(v any) float64                             { n, _ := v.(float64); return n }
func integer(v any) int32                           { return int32(num(v)) }
func ctx(c contractCase) (context.Context, context.CancelFunc) {
	pairs := []string{"x-conformance-case", c.ID}
	if auth := str(c.Input["auth"]); auth != "" && auth != "omitted" {
		pairs = append(pairs, "authorization", "Bearer "+auth)
	}
	base := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(pairs...))
	if c.ID == "error.deadline-exceeded" {
		return context.WithTimeout(base, 25*time.Millisecond)
	}
	if has(c.Expect.Semantics, "cancellationPropagates") {
		x, cancel := context.WithCancel(base)
		go func() { time.Sleep(10 * time.Millisecond); cancel() }()
		return x, cancel
	}
	return context.WithCancel(base)
}
func call(client pb.GeographyServiceClient, c contractCase) (any, error, any) {
	x, cancel := ctx(c)
	defer cancel()
	p, q := obj(c.Input, "path"), obj(c.Input, "query")
	var request proto.Message
	switch c.Operation {
	case "listRegions":
		r := &pb.ListRegionsRequest{Cursor: str(q["cursor"]), Limit: integer(q["limit"])}
		request = r
		v, e := client.ListRegions(x, r)
		return v, e, request
	case "getRegion":
		r := &pb.GetRegionRequest{Id: str(p["id"])}
		request = r
		v, e := client.GetRegion(x, r)
		return v, e, request
	case "listDistricts":
		r := &pb.ListDistrictsRequest{RegionId: str(q["regionId"]), Query: str(q["q"]), Cursor: str(q["cursor"]), Limit: integer(q["limit"])}
		request = r
		v, e := client.ListDistricts(x, r)
		return v, e, request
	case "getDistrict":
		r := &pb.GetDistrictRequest{Id: str(p["id"])}
		request = r
		v, e := client.GetDistrict(x, r)
		return v, e, request
	case "listPlaces":
		r := &pb.ListPlacesRequest{DistrictId: str(q["districtId"]), RegionId: str(q["regionId"]), Query: str(q["q"]), Cursor: str(q["cursor"]), Limit: integer(q["limit"])}
		request = r
		v, e := client.ListPlaces(x, r)
		return v, e, request
	case "getPlace":
		r := &pb.GetPlaceRequest{Id: str(p["id"])}
		request = r
		v, e := client.GetPlace(x, r)
		return v, e, request
	case "search":
		r := &pb.SearchRequest{Query: str(q["q"]), RegionId: str(q["regionId"]), DistrictId: str(q["districtId"]), Limit: integer(q["limit"])}
		request = r
		v, e := client.Search(x, r)
		return v, e, request
	case "autocomplete":
		r := &pb.AutocompleteRequest{Query: str(q["q"]), Limit: integer(q["limit"])}
		request = r
		v, e := client.Autocomplete(x, r)
		return v, e, request
	case "geocode":
		r := &pb.GeocodeRequest{Query: str(q["q"]), Limit: integer(q["limit"])}
		request = r
		v, e := client.Geocode(x, r)
		return v, e, request
	case "reverseGeocode":
		r := &pb.ReverseGeocodeRequest{Latitude: num(q["lat"]), Longitude: num(q["lng"])}
		request = r
		v, e := client.ReverseGeocode(x, r)
		return v, e, request
	case "nearby":
		r := &pb.NearbyRequest{Latitude: num(q["lat"]), Longitude: num(q["lng"]), RadiusMeters: integer(q["radius"]), Limit: integer(q["limit"])}
		request = r
		v, e := client.Nearby(x, r)
		return v, e, request
	case "getBoundary":
		r := &pb.GetBoundaryRequest{Id: str(p["id"])}
		request = r
		v, e := client.GetBoundary(x, r)
		return v, e, request
	case "streamDatasetChanges":
		r := &pb.StreamDatasetChangesRequest{SinceCursor: str(q["cursor"])}
		request = r
		s, e := client.StreamDatasetChanges(x, r)
		if e != nil {
			return nil, e, request
		}
		items := []*pb.StreamDatasetChangesResponse{}
		for {
			v, e := s.Recv()
			if e == io.EOF {
				break
			}
			if e != nil {
				return items, e, request
			}
			items = append(items, v)
		}
		if len(items) != 2 || items[0].GetChange().GetCursor() != "cursor-1" || items[1].GetChange().GetCursor() != "cursor-2" {
			return items, fmt.Errorf("initial change stream is not deterministic"), request
		}
		resumeContext := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-conformance-case", c.ID))
		resumed, e := client.StreamDatasetChanges(resumeContext, &pb.StreamDatasetChangesRequest{SinceCursor: items[0].GetChange().GetCursor()})
		if e != nil {
			return items, e, request
		}
		resumedItem, e := resumed.Recv()
		if e != nil || resumedItem.GetChange().GetCursor() != "cursor-2" {
			return items, fmt.Errorf("resumed stream did not continue after cursor-1"), request
		}
		if _, e = resumed.Recv(); e != io.EOF {
			return items, fmt.Errorf("resumed stream returned duplicate changes"), request
		}
		return map[string]any{"initial": items, "resumed": []*pb.StreamDatasetChangesResponse{resumedItem}}, nil, map[string]any{"initial": r, "resumeCursor": "cursor-1"}
	default:
		return nil, fmt.Errorf("unmapped gRPC operation %s", c.Operation), map[string]any{"operation": c.Operation}
	}
}
func validate(c contractCase, value any, err error) []string {
	fail := []string{}
	if has(c.Expect.Semantics, "cancellationPropagates") {
		if status.Code(err) != 1 {
			return []string{"cancellation did not propagate"}
		}
		return nil
	}
	if c.Expect.Outcome == "error" {
		if err == nil {
			return []string{"expected gRPC error"}
		}
		if status.Code(err) != grpcCode(c.Expect.Error.GrpcStatus) {
			fail = append(fail, fmt.Sprintf("expected %s, got %s", c.Expect.Error.GrpcStatus, status.Code(err)))
		}
		return fail
	}
	if err != nil {
		return []string{err.Error()}
	}
	encoded, _ := json.Marshal(value)
	if !strings.Contains(string(encoded), dataset) {
		fail = append(fail, "canonical dataset version absent")
	}
	if has(c.Expect.Semantics, "preservesGhanaianOrthography") && !strings.Contains(string(encoded), "Mampɔŋ") {
		fail = append(fail, "orthography not preserved")
	}
	if has(c.Expect.Semantics, "emptyOutsideGhana") && strings.Contains(string(encoded), "gh-region-") {
		fail = append(fail, "outside-Ghana response not empty")
	}
	if has(c.Expect.Semantics, "protocolResultsEquivalent") {
		searchResult, ok := value.(*pb.SearchResponse)
		if !ok || len(searchResult.Results) != 1 || searchResult.Results[0].GetPlace().GetId() != "gh-place-kumasi" || searchResult.Results[0].GetPlace().GetName() != "Kumasi" || searchResult.DatasetVersion != dataset {
			fail = append(fail, "gRPC semantic search result differs from canonical parity value")
		}
	}
	return fail
}
func grpcCode(value string) codes.Code {
	switch value {
	case "INVALID_ARGUMENT":
		return codes.InvalidArgument
	case "RESOURCE_EXHAUSTED":
		return codes.ResourceExhausted
	case "UNAUTHENTICATED":
		return codes.Unauthenticated
	case "PERMISSION_DENIED":
		return codes.PermissionDenied
	case "NOT_FOUND":
		return codes.NotFound
	case "DEADLINE_EXCEEDED":
		return codes.DeadlineExceeded
	case "INTERNAL":
		return codes.Internal
	default:
		return codes.Unknown
	}
}
func cursorProof(client pb.GeographyServiceClient, c contractCase) []string {
	x := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-conformance-case", c.ID))
	a, e := client.ListPlaces(x, &pb.ListPlacesRequest{Limit: 2})
	if e != nil {
		return []string{e.Error()}
	}
	if len(a.Places) != 1 || len(a.NextCursor) < 8 {
		return []string{"first cursor page invalid"}
	}
	b, e := client.ListPlaces(x, &pb.ListPlacesRequest{Limit: 2, Cursor: a.NextCursor})
	if e != nil || len(b.Places) != 1 || a.Places[0].Id == b.Places[0].Id {
		return []string{"second cursor page invalid"}
	}
	return nil
}
func digest(v any) string {
	var b []byte
	if m, ok := v.(proto.Message); ok {
		b, _ = protojson.Marshal(m)
	} else {
		b, _ = json.Marshal(v)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
