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
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/auth"
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

func unaryMiddleware(a *auth.Authenticator, log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		started := time.Now()
		requestID := fmt.Sprintf("grpc-%d", requestSequence.Add(1))
		ctx, cancel := context.WithTimeout(ctx, unaryTimeout)
		defer cancel()

		var response any
		resolved, _, err := authenticate(ctx, a, info.FullMethod)
		if err == nil {
			ctx = resolved
			response, err = handler(ctx, req)
		}
		log.Info("request", "request_id", requestID, "protocol", "grpc", "operation", info.FullMethod,
			"status", status.Code(err).String(), "latency_ms", time.Since(started).Milliseconds(),
			"caller", auth.FromContext(ctx).RateKey)
		return response, err
	}
}

type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextStream) Context() context.Context { return s.ctx }

func streamMiddleware(a *auth.Authenticator, log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		started := time.Now()
		ctx, cancel := context.WithTimeout(stream.Context(), streamTimeout)
		defer cancel()
		resolved, _, err := authenticate(ctx, a, info.FullMethod)
		if err == nil {
			err = handler(srv, &contextStream{ServerStream: stream, ctx: resolved})
		}
		log.Info("request", "request_id", fmt.Sprintf("grpc-%d", requestSequence.Add(1)), "protocol", "grpc",
			"operation", info.FullMethod, "status", status.Code(err).String(), "latency_ms", time.Since(started).Milliseconds(),
			"caller", auth.FromContext(resolved).RateKey)
		return err
	}
}
