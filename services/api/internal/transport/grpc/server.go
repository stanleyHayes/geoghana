// Package grpc is the gRPC transport adapter.
//
// It validates wire input, calls a use case, and maps the result back. There
// is no business logic here — the same application services back REST and
// GraphQL, so an equivalent query returns semantically identical results
// (Spec §6, CLAUDE.md rule 2).
package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/ghanageo/ghanageo/services/api/gen/ghanageo/v1"
	mongoadapter "github.com/ghanageo/ghanageo/services/api/internal/adapters/mongo"
	appgeo "github.com/ghanageo/ghanageo/services/api/internal/app/geography"
	appsearch "github.com/ghanageo/ghanageo/services/api/internal/app/search"
	usageDomain "github.com/ghanageo/ghanageo/services/api/internal/domain/usage"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/auth"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// Server implements ghanageo.v1.GeographyService.
type Server struct {
	pb.UnimplementedGeographyServiceServer
	geo    *appgeo.Service
	search *appsearch.Service
	store  *mongoadapter.Store
	log    *slog.Logger
}

func NewServer(
	geo *appgeo.Service, search *appsearch.Service,
	store *mongoadapter.Store, log *slog.Logger,
) *Server {
	return &Server{geo: geo, search: search, store: store, log: log}
}

// grpcCode maps the shared error catalog onto gRPC status codes, using the
// SAME mapping the catalog declares for REST. Two transports disagreeing about
// what an error means is how a client ends up retrying a permanent failure.
var grpcCode = map[apierr.Code]codes.Code{
	apierr.InvalidArgument:   codes.InvalidArgument,
	apierr.InvalidCoordinate: codes.InvalidArgument,
	apierr.RadiusOutOfRange:  codes.InvalidArgument,
	apierr.QueryTooShort:     codes.InvalidArgument,
	apierr.QueryTooComplex:   codes.InvalidArgument,
	apierr.PayloadTooLarge:   codes.ResourceExhausted,
	apierr.Unauthenticated:   codes.Unauthenticated,
	apierr.KeyRevoked:        codes.Unauthenticated,
	apierr.PermissionDenied:  codes.PermissionDenied,
	apierr.OriginNotAllowed:  codes.PermissionDenied,
	apierr.NotFound:          codes.NotFound,
	// A merged record is NOT_FOUND in gRPC, which has no 410. The message
	// carries the redirect so a client can still follow it.
	apierr.ResourceGone:      codes.NotFound,
	apierr.RateLimitExceeded: codes.ResourceExhausted,
	apierr.QuotaExceeded:     codes.ResourceExhausted,
	apierr.DeadlineExceeded:  codes.DeadlineExceeded,
	apierr.Internal:          codes.Internal,
}

// toStatus converts an application error into a gRPC status.
//
// An unrecognised error becomes INTERNAL with a generic message and is LOGGED
// with its cause: a 500 whose reason was never recorded is the hardest kind of
// bug to chase.
func (s *Server) toStatus(ctx context.Context, op string, err error) error {
	var ae *apierr.Error
	if errors.As(err, &ae) {
		c, ok := grpcCode[ae.Code]
		if !ok {
			c = codes.Internal
		}
		if c == codes.Internal {
			s.log.ErrorContext(ctx, "grpc internal error", "op", op, "code", ae.Code, "err", err)
		}
		// The machine code travels in the message, because a gRPC code alone
		// cannot distinguish RATE_LIMIT_EXCEEDED from QUOTA_EXCEEDED.
		return status.Errorf(c, "%s: %s", ae.Code, ae.Message)
	}
	s.log.ErrorContext(ctx, "grpc unhandled error", "op", op, "err", err)
	return status.Error(codes.Internal, "INTERNAL: An unexpected error occurred.")
}

// clampLimit keeps a caller from asking for the whole dataset in one message.
// Zero means "unset" in proto3, so it maps to the default rather than to zero
// results.
func clampLimit(n int32) int {
	switch {
	case n <= 0:
		return defaultLimit
	case int(n) > maxLimit:
		return maxLimit
	default:
		return int(n)
	}
}

func (s *Server) ListRegions(
	ctx context.Context, req *pb.ListRegionsRequest,
) (*pb.ListRegionsResponse, error) {
	page, err := s.geo.ListRegions(ctx, ports.ListParams{
		Cursor: req.GetCursor(), Limit: clampLimit(req.GetLimit()),
	})
	if err != nil {
		return nil, s.toStatus(ctx, "ListRegions", err)
	}
	out := make([]*pb.Region, 0, len(page.Data))
	for _, r := range page.Data {
		out = append(out, regionToPB(r))
	}
	return &pb.ListRegionsResponse{
		Regions: out, NextCursor: page.NextCursor, DatasetVersion: page.DatasetVersion,
	}, nil
}

func (s *Server) GetRegion(
	ctx context.Context, req *pb.GetRegionRequest,
) (*pb.GetRegionResponse, error) {
	r, err := s.geo.GetRegion(ctx, req.GetId())
	if err != nil {
		return nil, s.toStatus(ctx, "GetRegion", err)
	}
	return &pb.GetRegionResponse{Region: regionToPB(*r)}, nil
}

func (s *Server) ListDistricts(
	ctx context.Context, req *pb.ListDistrictsRequest,
) (*pb.ListDistrictsResponse, error) {
	page, err := s.geo.ListDistricts(ctx, ports.DistrictFilter{
		ListParams: ports.ListParams{Cursor: req.GetCursor(), Limit: clampLimit(req.GetLimit())},
		RegionID:   req.GetRegionId(),
		Query:      req.GetQuery(),
	})
	if err != nil {
		return nil, s.toStatus(ctx, "ListDistricts", err)
	}
	out := make([]*pb.District, 0, len(page.Data))
	for _, d := range page.Data {
		out = append(out, districtToPB(d))
	}
	return &pb.ListDistrictsResponse{
		Districts: out, NextCursor: page.NextCursor, DatasetVersion: page.DatasetVersion,
	}, nil
}

func (s *Server) GetDistrict(
	ctx context.Context, req *pb.GetDistrictRequest,
) (*pb.GetDistrictResponse, error) {
	d, err := s.geo.GetDistrict(ctx, req.GetId())
	if err != nil {
		return nil, s.toStatus(ctx, "GetDistrict", err)
	}
	return &pb.GetDistrictResponse{District: districtToPB(*d)}, nil
}

func (s *Server) ListPlaces(
	ctx context.Context, req *pb.ListPlacesRequest,
) (*pb.ListPlacesResponse, error) {
	page, err := s.geo.ListPlaces(ctx, ports.PlaceFilter{
		ListParams: ports.ListParams{Cursor: req.GetCursor(), Limit: clampLimit(req.GetLimit())},
		DistrictID: req.GetDistrictId(),
		RegionID:   req.GetRegionId(),
		Type:       placeTypeFromPB(req.GetType()),
		Query:      req.GetQuery(),
	})
	if err != nil {
		return nil, s.toStatus(ctx, "ListPlaces", err)
	}
	out := make([]*pb.Place, 0, len(page.Data))
	for _, p := range page.Data {
		out = append(out, placeToPB(p))
	}
	return &pb.ListPlacesResponse{
		Places: out, NextCursor: page.NextCursor, DatasetVersion: page.DatasetVersion,
	}, nil
}

func (s *Server) GetPlace(
	ctx context.Context, req *pb.GetPlaceRequest,
) (*pb.GetPlaceResponse, error) {
	p, err := s.geo.GetPlace(ctx, req.GetId())
	if err != nil {
		return nil, s.toStatus(ctx, "GetPlace", err)
	}
	return &pb.GetPlaceResponse{Place: placeToPB(*p)}, nil
}

func (s *Server) Search(
	ctx context.Context, req *pb.SearchRequest,
) (*pb.SearchResponse, error) {
	hits, err := s.search.Search(ctx, ports.SearchQuery{
		Text:       req.GetQuery(),
		Limit:      clampLimit(req.GetLimit()),
		RegionID:   req.GetRegionId(),
		DistrictID: req.GetDistrictId(),
		Type:       placeTypeFromPB(req.GetType()),
	})
	if err != nil {
		return nil, s.toStatus(ctx, "Search", err)
	}
	return &pb.SearchResponse{
		Results: hitsToPB(hits), DatasetVersion: s.search.DatasetVersion(),
	}, nil
}

func (s *Server) Autocomplete(
	ctx context.Context, req *pb.AutocompleteRequest,
) (*pb.AutocompleteResponse, error) {
	hits, err := s.search.Autocomplete(ctx, req.GetQuery(), clampLimit(req.GetLimit()))
	if err != nil {
		return nil, s.toStatus(ctx, "Autocomplete", err)
	}
	return &pb.AutocompleteResponse{
		Suggestions: hitsToPB(hits), DatasetVersion: s.search.DatasetVersion(),
	}, nil
}

func (s *Server) Geocode(
	ctx context.Context, req *pb.GeocodeRequest,
) (*pb.GeocodeResponse, error) {
	hits, err := s.search.Geocode(ctx, req.GetQuery(), clampLimit(req.GetLimit()))
	if err != nil {
		return nil, s.toStatus(ctx, "Geocode", err)
	}
	return &pb.GeocodeResponse{
		Results: hitsToPB(hits), DatasetVersion: s.search.DatasetVersion(),
	}, nil
}

func (s *Server) ReverseGeocode(
	ctx context.Context, req *pb.ReverseGeocodeRequest,
) (*pb.ReverseGeocodeResponse, error) {
	res, err := s.search.Reverse(ctx, req.GetLatitude(), req.GetLongitude())
	if err != nil {
		return nil, s.toStatus(ctx, "ReverseGeocode", err)
	}
	out := &pb.ReverseGeocodeResponse{DatasetVersion: res.DatasetVersion}
	if res.Region != nil {
		out.Region = refToPB(res.Region.ID, res.Region.Name)
	}
	if res.District != nil {
		out.District = refToPB(res.District.ID, res.District.Name)
	}
	for _, p := range res.Nearby {
		out.Nearby = append(out.Nearby, placeToPB(p))
	}
	return out, nil
}

func (s *Server) Nearby(
	ctx context.Context, req *pb.NearbyRequest,
) (*pb.NearbyResponse, error) {
	places, err := s.geo.Nearby(
		ctx, req.GetLatitude(), req.GetLongitude(),
		int(req.GetRadiusMeters()), clampLimit(req.GetLimit()),
	)
	if err != nil {
		return nil, s.toStatus(ctx, "Nearby", err)
	}
	out := make([]*pb.Place, 0, len(places))
	for _, p := range places {
		out = append(out, placeToPB(p))
	}
	return &pb.NearbyResponse{Places: out, DatasetVersion: s.geo.DatasetVersion()}, nil
}

func (s *Server) GetBoundary(
	ctx context.Context, req *pb.GetBoundaryRequest,
) (*pb.GetBoundaryResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unimplemented, "Boundary lookup is not configured.")
	}
	b, err := s.store.FindBoundary(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: No record matches that identifier.", apierr.NotFound)
	}
	if b.Geometry == nil {
		// The record exists, its geometry has not been ingested. Saying so
		// beats a bare NOT_FOUND, which would suggest the id is wrong.
		return nil, status.Errorf(codes.NotFound,
			"%s: This record has no boundary geometry yet (%s).", apierr.NotFound, b.Name)
	}
	geo, err := json.Marshal(b.Geometry)
	if err != nil {
		return nil, s.toStatus(ctx, "GetBoundary", fmt.Errorf("marshal geometry: %w", err))
	}
	return &pb.GetBoundaryResponse{Boundary: &pb.Boundary{
		Id:      b.ID,
		Name:    b.Name,
		Geojson: string(geo),
		// Attribution travels WITH the data, as CC BY requires. A footer on a
		// website does not satisfy the licence for an API response.
		Attribution:    "Boundaries from geoBoundaries (https://www.geoboundaries.org), licensed CC BY 4.0.",
		DatasetVersion: b.DatasetVersion,
	}}, nil
}

// StreamDatasetChanges is declared in the published contract but has no
// backing change feed yet (GEO-8.3).
//
// It returns UNIMPLEMENTED rather than an empty stream that never yields:
// a client cannot tell "no changes" from "not built", and a silent stream
// would have them waiting forever on a feature that does not exist.
func (s *Server) StreamDatasetChanges(
	_ *pb.StreamDatasetChangesRequest, _ pb.GeographyService_StreamDatasetChangesServer,
) error {
	return status.Error(codes.Unimplemented,
		"StreamDatasetChanges is not implemented yet — it needs the dataset change feed (GEO-8.3).")
}

func hitsToPB(hits []ports.SearchHit) []*pb.SearchResult {
	out := make([]*pb.SearchResult, 0, len(hits))
	for _, h := range hits {
		out = append(out, searchHitToPB(h))
	}
	return out
}

// Serve starts the gRPC server on addr and blocks until ctx is cancelled.
func Serve(ctx context.Context, addr string, s *Server, authenticator *auth.Authenticator, log *slog.Logger, usageRepository ...usageDomain.Repository) error {
	var repository usageDomain.Repository
	if len(usageRepository) > 0 {
		repository = usageRepository[0]
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("grpc listen %s: %w", addr, err)
	}

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(unaryMiddleware(authenticator, log, repository)),
		grpc.ChainStreamInterceptor(streamMiddleware(authenticator, log, repository)),
		grpc.MaxRecvMsgSize(maxMessage),
		grpc.MaxSendMsgSize(maxMessage),
	)
	pb.RegisterGeographyServiceServer(srv, s)

	// Health and reflection: reflection is what lets grpcurl and Postman
	// explore the API without a local copy of the .proto, which matters for a
	// public interface people are meant to try before committing to it.
	hs := health.NewServer()
	hs.SetServingStatus("ghanageo.v1.GeographyService", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)
	reflection.Register(srv)

	go func() {
		<-ctx.Done()
		srv.GracefulStop()
	}()

	log.Info("grpc listening", "addr", addr)
	if err := srv.Serve(lis); err != nil {
		return fmt.Errorf("grpc serve: %w", err)
	}
	return nil
}
