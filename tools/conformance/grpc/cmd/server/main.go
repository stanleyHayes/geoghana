package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	pb "github.com/ghanageo/ghanageo-go/proto/ghanageo/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const dataset = "2026.08.3-ulid"

type contractCase struct {
	ID, Operation string
	Protocols     []string
	Input         map[string]any
}
type corpus struct{ Cases []contractCase }
type fixture struct {
	pb.UnimplementedGeographyServiceServer
	cases map[string]contractCase
}

func main() {
	casesPath := flag.String("cases", "", "exported conformance cases JSON")
	flag.Parse()
	raw, err := os.ReadFile(*casesPath)
	if err != nil {
		panic(err)
	}
	var all corpus
	if err := json.Unmarshal(raw, &all); err != nil {
		panic(err)
	}
	indexed := map[string]contractCase{}
	for _, c := range all.Cases {
		indexed[c.ID] = c
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	server := grpc.NewServer()
	pb.RegisterGeographyServiceServer(server, &fixture{cases: indexed})
	fmt.Println(listener.Addr().(*net.TCPAddr).Port)
	if err := server.Serve(listener); err != nil {
		panic(err)
	}
}

func (f *fixture) check(ctx context.Context, operation string) (contractCase, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	id := strings.Join(md.Get("x-conformance-case"), "")
	c, ok := f.cases[id]
	if !ok {
		return c, status.Error(codes.InvalidArgument, "unknown conformance case")
	}
	if c.Operation != operation || !contains(c.Protocols, "grpc") {
		return c, status.Error(codes.InvalidArgument, "case/operation protocol mismatch")
	}
	authorization := strings.Join(md.Get("authorization"), "")
	if wanted, ok := c.Input["auth"].(string); ok && wanted != "omitted" && authorization != "Bearer "+wanted {
		return c, status.Error(codes.InvalidArgument, "authorization fixture mismatch")
	}
	if (c.Input["auth"] == nil || c.Input["auth"] == "omitted") && authorization != "" {
		return c, status.Error(codes.InvalidArgument, "unexpected authorization")
	}
	_ = grpc.SetHeader(ctx, metadata.Pairs("x-conformance-fixture", "smoke-only"))
	if id == "semantic.cancellation-propagates" {
		select {
		case <-time.After(150 * time.Millisecond):
		case <-ctx.Done():
			return c, status.Error(codes.Canceled, "request cancelled")
		}
	}
	return c, nil
}
func catalogFailure(c contractCase) error {
	if !strings.HasPrefix(c.ID, "error.") {
		return nil
	}
	if c.ID == "error.deadline-exceeded" {
		time.Sleep(150 * time.Millisecond)
	}
	return status.Error(errorCode(c.ID), c.ID)
}
func wire(c contractCase, actual map[string]any) error {
	for _, section := range []string{"path", "query"} {
		expected, _ := c.Input[section].(map[string]any)
		for key, wanted := range expected {
			if key == "cursor" && fmt.Sprint(wanted) == "" && fmt.Sprint(actual[key]) != "" && (c.ID == "semantic.cursor-pagination" || c.Operation == "streamDatasetChanges") {
				continue
			}
			if fmt.Sprint(actual[key]) != fmt.Sprint(wanted) {
				return status.Errorf(codes.InvalidArgument, "%s.%s fixture input mismatch", section, key)
			}
		}
	}
	if wanted, exists := c.Input["cursor"]; exists && fmt.Sprint(actual["cursor"]) != fmt.Sprint(wanted) && !(c.Operation == "streamDatasetChanges" && fmt.Sprint(actual["cursor"]) == "cursor-1") {
		return status.Error(codes.InvalidArgument, "cursor fixture input mismatch")
	}
	return nil
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func errorCode(id string) codes.Code {
	switch id {
	case "error.invalid-argument", "error.invalid-coordinates", "error.radius-out-of-range", "error.query-too-short":
		return codes.InvalidArgument
	case "error.payload-too-large", "error.rate-limit-exceeded", "error.quota-exceeded":
		return codes.ResourceExhausted
	case "error.unauthenticated", "error.key-revoked":
		return codes.Unauthenticated
	case "error.permission-denied", "error.origin-not-allowed":
		return codes.PermissionDenied
	case "error.not-found", "error.resource-gone":
		return codes.NotFound
	case "error.deadline-exceeded":
		return codes.DeadlineExceeded
	default:
		return codes.Internal
	}
}
func region() *pb.Region {
	return &pb.Region{Id: "gh-region-ashanti", CountryCode: "GH", Name: "Ashanti", Status: pb.Status_STATUS_ACTIVE, VerificationStatus: pb.VerificationStatus_VERIFICATION_STATUS_REFERENCE, DatasetVersion: dataset, Provenance: &pb.Provenance{SourceId: "fixture"}}
}
func district() *pb.District {
	return &pb.District{Id: "gh-district-ahafo-asunafo-north", Name: "Asunafo North", Region: &pb.Ref{Id: "gh-region-ahafo", Name: "Ahafo"}, Status: pb.Status_STATUS_ACTIVE, VerificationStatus: pb.VerificationStatus_VERIFICATION_STATUS_REFERENCE, DatasetVersion: dataset, Provenance: &pb.Provenance{SourceId: "fixture"}}
}
func place(name string) *pb.Place {
	return &pb.Place{Id: "gh-place-kumasi", Name: name, NormalizedName: "kumasi", Type: pb.PlaceType_PLACE_TYPE_CITY, Status: pb.Status_STATUS_ACTIVE, VerificationStatus: pb.VerificationStatus_VERIFICATION_STATUS_REFERENCE, DatasetVersion: dataset, Provenance: &pb.Provenance{SourceId: "fixture"}}
}
func search(name string) []*pb.SearchResult {
	return []*pb.SearchResult{{Place: place(name), Score: 1, MatchReason: "fixture"}}
}

func (f *fixture) ListRegions(ctx context.Context, r *pb.ListRegionsRequest) (*pb.ListRegionsResponse, error) {
	c, e := f.check(ctx, "listRegions")
	if e == nil {
		e = wire(c, map[string]any{"cursor": r.Cursor, "limit": r.Limit})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return nil, e
	}
	out := &pb.ListRegionsResponse{Regions: []*pb.Region{region()}, DatasetVersion: dataset}
	if c.ID == "semantic.cursor-pagination" && r.Cursor == "" {
		out.NextCursor = "eyJvZmZzZXQiOjJ9"
	}
	return out, nil
}
func (f *fixture) GetRegion(ctx context.Context, r *pb.GetRegionRequest) (*pb.GetRegionResponse, error) {
	c, e := f.check(ctx, "getRegion")
	if e == nil {
		e = wire(c, map[string]any{"id": r.Id})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return nil, e
	}
	return &pb.GetRegionResponse{Region: region()}, nil
}
func (f *fixture) ListDistricts(ctx context.Context, r *pb.ListDistrictsRequest) (*pb.ListDistrictsResponse, error) {
	c, e := f.check(ctx, "listDistricts")
	if e == nil {
		e = wire(c, map[string]any{"regionId": r.RegionId, "q": r.Query, "cursor": r.Cursor, "limit": r.Limit})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return nil, e
	}
	return &pb.ListDistrictsResponse{Districts: []*pb.District{district()}, DatasetVersion: dataset}, nil
}
func (f *fixture) GetDistrict(ctx context.Context, r *pb.GetDistrictRequest) (*pb.GetDistrictResponse, error) {
	c, e := f.check(ctx, "getDistrict")
	if e == nil {
		e = wire(c, map[string]any{"id": r.Id})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return nil, e
	}
	return &pb.GetDistrictResponse{District: district()}, nil
}
func (f *fixture) ListPlaces(ctx context.Context, r *pb.ListPlacesRequest) (*pb.ListPlacesResponse, error) {
	c, e := f.check(ctx, "listPlaces")
	if e == nil {
		e = wire(c, map[string]any{"districtId": r.DistrictId, "regionId": r.RegionId, "q": r.Query, "cursor": r.Cursor, "limit": r.Limit})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return nil, e
	}
	p := place("Kumasi")
	if c.ID == "semantic.autocomplete-orthography" {
		p = place("Mampɔŋ")
	}
	if c.ID == "semantic.cursor-pagination" {
		if r.Cursor == "" {
			p.Id = "gh-place-page-1"
		} else {
			p.Id = "gh-place-page-2"
		}
	}
	out := &pb.ListPlacesResponse{Places: []*pb.Place{p}, DatasetVersion: dataset}
	if c.ID == "semantic.cursor-pagination" && r.Cursor == "" {
		out.NextCursor = "eyJvZmZzZXQiOjJ9"
	}
	return out, nil
}
func (f *fixture) GetPlace(ctx context.Context, r *pb.GetPlaceRequest) (*pb.GetPlaceResponse, error) {
	c, e := f.check(ctx, "getPlace")
	if e == nil {
		e = wire(c, map[string]any{"id": r.Id})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return nil, e
	}
	return &pb.GetPlaceResponse{Place: place("Kumasi")}, nil
}
func (f *fixture) Search(ctx context.Context, r *pb.SearchRequest) (*pb.SearchResponse, error) {
	c, e := f.check(ctx, "search")
	if e == nil {
		e = wire(c, map[string]any{"q": r.Query, "regionId": r.RegionId, "districtId": r.DistrictId, "limit": r.Limit})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return nil, e
	}
	return &pb.SearchResponse{Results: search("Kumasi"), DatasetVersion: dataset}, nil
}
func (f *fixture) Autocomplete(ctx context.Context, r *pb.AutocompleteRequest) (*pb.AutocompleteResponse, error) {
	c, e := f.check(ctx, "autocomplete")
	if e == nil {
		e = wire(c, map[string]any{"q": r.Query, "limit": r.Limit})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return nil, e
	}
	name := "Kumasi"
	if c.ID == "semantic.autocomplete-orthography" {
		name = "Mampɔŋ"
	}
	return &pb.AutocompleteResponse{Suggestions: search(name), DatasetVersion: dataset}, nil
}
func (f *fixture) Geocode(ctx context.Context, r *pb.GeocodeRequest) (*pb.GeocodeResponse, error) {
	c, e := f.check(ctx, "geocode")
	if e == nil {
		e = wire(c, map[string]any{"q": r.Query, "limit": r.Limit})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return nil, e
	}
	return &pb.GeocodeResponse{Results: search("Kumasi"), DatasetVersion: dataset}, nil
}
func (f *fixture) ReverseGeocode(ctx context.Context, r *pb.ReverseGeocodeRequest) (*pb.ReverseGeocodeResponse, error) {
	c, e := f.check(ctx, "reverseGeocode")
	if e == nil {
		e = wire(c, map[string]any{"lat": r.Latitude, "lng": r.Longitude})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return nil, e
	}
	if c.ID == "semantic.reverse-outside-ghana" || r.Latitude > 20 {
		return &pb.ReverseGeocodeResponse{DatasetVersion: dataset}, nil
	}
	return &pb.ReverseGeocodeResponse{Region: &pb.Ref{Id: "gh-region-ashanti", Name: "Ashanti"}, DatasetVersion: dataset}, nil
}
func (f *fixture) Nearby(ctx context.Context, r *pb.NearbyRequest) (*pb.NearbyResponse, error) {
	c, e := f.check(ctx, "nearby")
	if e == nil {
		e = wire(c, map[string]any{"lat": r.Latitude, "lng": r.Longitude, "radius": r.RadiusMeters, "limit": r.Limit})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return nil, e
	}
	return &pb.NearbyResponse{Places: []*pb.Place{place("Kumasi")}, DatasetVersion: dataset}, nil
}
func (f *fixture) GetBoundary(ctx context.Context, r *pb.GetBoundaryRequest) (*pb.GetBoundaryResponse, error) {
	c, e := f.check(ctx, "getBoundary")
	if e == nil {
		e = wire(c, map[string]any{"id": r.Id})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return nil, e
	}
	return &pb.GetBoundaryResponse{Boundary: &pb.Boundary{Id: "gh-region-ashanti", Name: "Ashanti", Geojson: `{"type":"Polygon","coordinates":[]}`, Attribution: "fixture", DatasetVersion: dataset}}, nil
}
func (f *fixture) StreamDatasetChanges(r *pb.StreamDatasetChangesRequest, stream grpc.ServerStreamingServer[pb.StreamDatasetChangesResponse]) error {
	c, e := f.check(stream.Context(), "streamDatasetChanges")
	if e == nil {
		e = wire(c, map[string]any{"cursor": r.SinceCursor})
	}
	if e == nil {
		e = catalogFailure(c)
	}
	if e != nil {
		return e
	}
	changes := []*pb.DatasetChange{{Cursor: "cursor-1", ChangeType: pb.DatasetChange_CHANGE_TYPE_ADDED, EntityType: "place", EntityId: "gh-place-1", DatasetVersion: dataset, OccurredAt: "2026-08-30T00:00:00Z"}, {Cursor: "cursor-2", ChangeType: pb.DatasetChange_CHANGE_TYPE_UPDATED, EntityType: "place", EntityId: "gh-place-2", DatasetVersion: dataset, OccurredAt: "2026-08-30T00:01:00Z"}}
	start := 0
	if r.SinceCursor == "cursor-1" {
		start = 1
	}
	for _, change := range changes[start:] {
		if err := stream.Send(&pb.StreamDatasetChangesResponse{Change: change}); err != nil {
			return err
		}
	}
	return nil
}
