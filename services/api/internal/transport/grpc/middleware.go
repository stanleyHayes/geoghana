package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	usageDomain "github.com/ghanageo/ghanageo/services/api/internal/domain/usage"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/auth"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const (
	unaryTimeout  = 15 * time.Second
	streamTimeout = 5 * time.Minute
	maxMessage    = 1 << 20
)

var requestSequence atomic.Uint64

func grpcCost(method string) identity.CostClass {
	switch {
	case strings.HasSuffix(method, "/GetBoundary"):
		return identity.CostGeometry
	case strings.HasSuffix(method, "/ReverseGeocode"), strings.HasSuffix(method, "/Nearby"):
		return identity.CostSpatial
	case strings.HasSuffix(method, "/Search"), strings.HasSuffix(method, "/Autocomplete"),
		strings.HasSuffix(method, "/Geocode"), strings.HasSuffix(method, "/StreamDatasetChanges"):
		return identity.CostNormal
	default:
		return identity.CostCheap
	}
}

func systemMethod(method string) bool {
	return strings.HasPrefix(method, "/grpc.health.v1.Health/") ||
		strings.Contains(method, "ServerReflection/")
}

func requestMetadata(ctx context.Context) (credential, origin, ip string) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		credential = first(md.Get("authorization"))
		origin = first(md.Get("origin"))
		ip = first(md.Get("x-forwarded-for"))
		if before, _, ok := strings.Cut(ip, ","); ok {
			ip = strings.TrimSpace(before)
		}
	}
	if ip == "" {
		if caller, ok := peer.FromContext(ctx); ok {
			host, _, err := net.SplitHostPort(caller.Addr.String())
			if err == nil {
				ip = host
			} else {
				ip = caller.Addr.String()
			}
		}
	}
	if ip == "" {
		ip = "unknown"
	}
	return credential, origin, ip
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func limitHeaders(decision auth.Decision, cost identity.CostClass) metadata.MD {
	return metadata.Pairs(
		"x-ratelimit-limit", strconv.Itoa(decision.Limit),
		"x-ratelimit-remaining", strconv.Itoa(max(0, decision.Remaining)),
		"x-ratelimit-cost", strconv.Itoa(cost.Units()),
	)
}

func authenticate(ctx context.Context, a *auth.Authenticator, method string) (context.Context, auth.Decision, error) {
	credential, origin, ip := requestMetadata(ctx)
	cost := grpcCost(method)
	resolved, decision, err := a.Authorize(ctx, credential, ip, origin, cost)
	if decision.Limit > 0 {
		_ = grpc.SetHeader(ctx, limitHeaders(decision, cost))
	}
	if err != nil {
		e := apierr.From(err)
		code, ok := grpcCode[e.Code]
		if !ok {
			code = grpcCode[apierr.Internal]
		}
		return ctx, decision, status.Errorf(code, "%s: %s", e.Code, e.Message)
	}
	caller := auth.FromContext(resolved)
	if !caller.Anonymous && !caller.HasScope(identity.ScopeGRPCAccess) {
		return ctx, decision, status.Errorf(codes.PermissionDenied, "%s: This key lacks the required scope.", apierr.PermissionDenied)
	}
	return resolved, decision, nil
}

func recordUsage(ctx context.Context, repository usageDomain.Repository, requestID, protocol, operation, result string, geography string, latency time.Duration, decision auth.Decision, cost identity.CostClass) {
	caller := auth.FromContext(ctx)
	if repository == nil || caller.Key == nil {
		return
	}
	id, err := identity.NewID("use")
	if err != nil {
		return
	}
	recordCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = repository.Record(recordCtx, usageDomain.Event{ID: id, RequestID: requestID, OrganizationID: caller.Key.OrganizationID, ApplicationID: caller.Key.ApplicationID, KeyID: caller.Key.ID, Protocol: protocol, Operation: operation, Status: result, Success: result == codes.OK.String(), LatencyMS: latency.Milliseconds(), QuotaCost: cost.Units(), QuotaLimit: decision.Limit, QuotaRemaining: decision.Remaining, Geography: geography, At: time.Now().UTC()})
}

func unaryMiddleware(a *auth.Authenticator, log *slog.Logger, usageRepository ...usageDomain.Repository) grpc.UnaryServerInterceptor {
	var repository usageDomain.Repository
	if len(usageRepository) > 0 {
		repository = usageRepository[0]
	}
	return unaryMiddlewareObserved(a, log, repository, nil)
}

func unaryMiddlewareObserved(a *auth.Authenticator, log *slog.Logger, repository usageDomain.Repository, observed *observability.Telemetry) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		started := time.Now()
		requestID := grpcRequestID()
		ctx, cancel := context.WithTimeout(ctx, unaryTimeout)
		defer cancel()

		var response any
		resolved := ctx
		var err error
		var decision auth.Decision
		if !systemMethod(info.FullMethod) {
			resolved, decision, err = authenticate(ctx, a, info.FullMethod)
		}
		if err == nil {
			ctx = resolved
			response, err = handler(ctx, req)
		}
		elapsed := time.Since(started)
		result := status.Code(err).String()
		caller := auth.FromContext(ctx)
		appID, keyPrefix := callerFields(caller)
		if observed != nil {
			observed.ObserveRequest("grpc", info.FullMethod, result, elapsed)
		}
		log.Info("request", "request_id", requestID, "trace_id", observability.TraceID(ctx), "app_id", appID, "key_prefix", keyPrefix, "protocol", "grpc", "operation", info.FullMethod,
			"status", result, "latency_ms", elapsed.Milliseconds(),
			"caller", auth.FromContext(ctx).RateKey)
		recordUsage(resolved, repository, requestID, "grpc", info.FullMethod, status.Code(err).String(), "", time.Since(started), decision, grpcCost(info.FullMethod))
		return response, err
	}
}

func grpcRequestID() string {
	if id, err := identity.NewID("grpc"); err == nil {
		return id
	}
	return fmt.Sprintf("grpc-%d-%d", time.Now().UnixMilli(), requestSequence.Add(1))
}

type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextStream) Context() context.Context { return s.ctx }

// chargingStream accounts for every delivered stream message in addition to
// the connection charge applied by streamMiddleware. A long-lived consumer
// must not bypass the same fair-use budget paid by equivalent unary reads.
type chargingStream struct {
	*contextStream
	a *auth.Authenticator
}

func (s *chargingStream) SendMsg(message any) error {
	if _, _, err := authenticate(s.Context(), s.a, "/ghanageo.v1.GeographyService/ListRegions"); err != nil {
		return err
	}
	return s.ServerStream.SendMsg(message)
}

func streamMiddleware(a *auth.Authenticator, log *slog.Logger, usageRepository ...usageDomain.Repository) grpc.StreamServerInterceptor {
	var repository usageDomain.Repository
	if len(usageRepository) > 0 {
		repository = usageRepository[0]
	}
	return streamMiddlewareObserved(a, log, repository, nil)
}

func streamMiddlewareObserved(a *auth.Authenticator, log *slog.Logger, repository usageDomain.Repository, observed *observability.Telemetry) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		started := time.Now()
		ctx, cancel := context.WithTimeout(stream.Context(), streamTimeout)
		defer cancel()
		resolved := ctx
		var err error
		var decision auth.Decision
		if !systemMethod(info.FullMethod) {
			resolved, decision, err = authenticate(ctx, a, info.FullMethod)
		}
		if err == nil {
			wrapped := &contextStream{ServerStream: stream, ctx: resolved}
			if strings.HasSuffix(info.FullMethod, "/StreamDatasetChanges") {
				err = handler(srv, &chargingStream{contextStream: wrapped, a: a})
			} else {
				err = handler(srv, wrapped)
			}
		}
		requestID := grpcRequestID()
		elapsed := time.Since(started)
		result := status.Code(err).String()
		caller := auth.FromContext(resolved)
		appID, keyPrefix := callerFields(caller)
		if observed != nil {
			observed.ObserveRequest("grpc", info.FullMethod, result, elapsed)
		}
		log.Info("request", "request_id", requestID, "trace_id", observability.TraceID(resolved), "app_id", appID, "key_prefix", keyPrefix, "protocol", "grpc",
			"operation", info.FullMethod, "status", result, "latency_ms", elapsed.Milliseconds(),
			"caller", auth.FromContext(resolved).RateKey)
		recordUsage(resolved, repository, requestID, "grpc", info.FullMethod, status.Code(err).String(), "", time.Since(started), decision, grpcCost(info.FullMethod))
		return err
	}
}

func callerFields(caller identity.Identity) (string, string) {
	if caller.Key == nil {
		return "", ""
	}
	return caller.Key.ApplicationID, caller.Key.Prefix
}
