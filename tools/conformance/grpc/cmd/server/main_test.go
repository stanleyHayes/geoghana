package main

import (
	"context"
	"testing"

	pb "github.com/ghanageo/ghanageo-go/proto/ghanageo/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestCaseHeaderCannotOverrideWireInput(t *testing.T) {
	testCase := contractCase{ID: "operation.list-regions", Operation: "listRegions", Protocols: []string{"grpc"}, Input: map[string]any{"query": map[string]any{"limit": float64(2)}}}
	server := &fixture{cases: map[string]contractCase{testCase.ID: testCase}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-conformance-case", testCase.ID))
	if _, err := server.ListRegions(ctx, &pb.ListRegionsRequest{Limit: 99}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("wrong wire input was not rejected: %v", err)
	}
	if _, err := server.ListRegions(ctx, &pb.ListRegionsRequest{Limit: 2}); err != nil {
		t.Fatalf("canonical wire input was rejected: %v", err)
	}
}
