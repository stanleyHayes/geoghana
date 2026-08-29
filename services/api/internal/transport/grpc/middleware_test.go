package grpc

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	platformauth "github.com/ghanageo/ghanageo/services/api/internal/platform/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type testKeys struct{ key *identity.APIKey }

func (k testKeys) ByPrefix(context.Context, string) (*identity.APIKey, error) { return k.key, nil }
func (testKeys) MarkUsed(context.Context, string, time.Time) error            { return nil }

type testLimiter struct {
	decision platformauth.Decision
	cost     identity.CostClass
}

func (l *testLimiter) Allow(_ context.Context, _ identity.Identity, _ identity.Allowance, cost identity.CostClass) (platformauth.Decision, error) {
	l.cost = cost
	return l.decision, nil
}

func TestUnaryMiddlewareAuthenticatesLimitsAndAddsDeadline(t *testing.T) {
	limiter := &testLimiter{decision: platformauth.Decision{Allowed: true, Limit: 120, Remaining: 116}}
	a := platformauth.New(testKeys{}, limiter, false)
	interceptor := unaryMiddleware(a, discardLogger())
	called := 0

	response, err := interceptor(context.Background(), "request", &grpc.UnaryServerInfo{
		FullMethod: "/ghanageo.v1.GeographyService/Nearby",
	}, func(ctx context.Context, req any) (any, error) {
		called++
		if !platformauth.FromContext(ctx).Anonymous {
			t.Error("request without a credential was not deliberately anonymous")
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > unaryTimeout {
			t.Error("unary RPC did not receive the server deadline")
		}
		return req, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if response != "request" || called != 1 {
		t.Fatalf("handler result=%v calls=%d, want request and exactly one call", response, called)
	}
	if limiter.cost != identity.CostSpatial {
		t.Fatalf("Nearby cost = %q, want spatial", limiter.cost)
	}
}

func TestUnaryMiddlewareRejectsRateLimitBeforeHandler(t *testing.T) {
	limiter := &testLimiter{decision: platformauth.Decision{Allowed: false, Limit: 120, RetryAfter: 2}}
	a := platformauth.New(testKeys{}, limiter, false)
	called := false
	_, err := unaryMiddleware(a, discardLogger())(context.Background(), nil, &grpc.UnaryServerInfo{
		FullMethod: "/ghanageo.v1.GeographyService/Search",
	}, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	if called {
		t.Fatal("rate-limited request reached the handler")
	}
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("rate-limit code = %s, want RESOURCE_EXHAUSTED", status.Code(err))
	}
}

func TestUnaryMiddlewareRequiresGRPCScopeForAKey(t *testing.T) {
	generated, err := identity.Generate(identity.EnvTest)
	if err != nil {
		t.Fatal(err)
	}
	key := &identity.APIKey{
		Prefix: generated.Prefix, SecretHash: generated.SecretHash,
		Scopes: []identity.Scope{identity.ScopeLocationsRead},
	}
	limiter := &testLimiter{decision: platformauth.Decision{Allowed: true, Limit: 600}}
	a := platformauth.New(testKeys{key: key}, limiter, false)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+generated.Full))
	_, err = unaryMiddleware(a, discardLogger())(ctx, nil, &grpc.UnaryServerInfo{
		FullMethod: "/ghanageo.v1.GeographyService/ListRegions",
	}, func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("missing grpc:access code = %s, want PERMISSION_DENIED", status.Code(err))
	}
}

func TestGRPCCostClasses(t *testing.T) {
	cases := map[string]identity.CostClass{
		"GetRegion": identity.CostCheap, "Search": identity.CostNormal,
		"ReverseGeocode": identity.CostSpatial, "GetBoundary": identity.CostGeometry,
	}
	for method, want := range cases {
		if got := grpcCost("/ghanageo.v1.GeographyService/" + method); got != want {
			t.Errorf("%s cost = %q, want %q", method, got, want)
		}
	}
}

func TestHealthAndReflectionBypassConsumerQuota(t *testing.T) {
	limiter := &testLimiter{decision: platformauth.Decision{Allowed: false, Limit: 1}}
	a := platformauth.New(testKeys{}, limiter, false)
	called := false
	_, err := unaryMiddleware(a, discardLogger())(context.Background(), nil, &grpc.UnaryServerInfo{
		FullMethod: "/grpc.health.v1.Health/Check",
	}, func(context.Context, any) (any, error) {
		called = true
		return "serving", nil
	})
	if err != nil || !called {
		t.Fatalf("health probe was subject to consumer quota: called=%v err=%v", called, err)
	}
}

func TestGRPCServerRejectsOversizedMessages(t *testing.T) {
	listener := bufconn.Listen(2 * maxMessage)
	a := platformauth.New(testKeys{}, &testLimiter{decision: platformauth.Decision{Allowed: true, Limit: 120}}, false)
	server := grpc.NewServer(serverOptions(a, discardLogger(), nil)...)
	healthpb.RegisterHealthServer(server, health.NewServer())
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	_, err = healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{Service: strings.Repeat("x", maxMessage+1)})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("oversized request code = %s, want RESOURCE_EXHAUSTED; err=%v", status.Code(err), err)
	}
}
