package main

import "testing"

func TestEnabledTransports(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode       string
		http, grpc bool
		wantErr    bool
	}{
		{mode: "all", http: true, grpc: true},
		{mode: "HTTP", http: true},
		{mode: " grpc ", grpc: true},
		{mode: "invalid", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			httpEnabled, grpcEnabled, err := enabledTransports(tc.mode)
			if (err != nil) != tc.wantErr || httpEnabled != tc.http || grpcEnabled != tc.grpc {
				t.Fatalf("enabledTransports(%q) = (%v, %v, %v)", tc.mode, httpEnabled, grpcEnabled, err)
			}
		})
	}
}
