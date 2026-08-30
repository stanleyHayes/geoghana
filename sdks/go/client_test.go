package ghanageo

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.HandlerFunc, options ...Option) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	all := append([]Option{WithBaseURL(server.URL), withSleep(func(context.Context, time.Duration) error { return nil })}, options...)
	client, err := New(all...)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	return client, server
}

func TestAPIKeyAndLocalNoThrowTelemetry(t *testing.T) {
	events := 0
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer caller-key" {
			t.Errorf("authorization=%q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-GhanaGeo-SDK") != "" {
			t.Error("telemetry leaked over network")
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}, WithAPIKey("caller-key"), WithTelemetry(func(event TelemetryEvent) { events++; panic("observer failure") }))
	defer server.Close()
	if _, err := client.Regions(context.Background(), PageOptions{}); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("events=%d", events)
	}
}

func TestRetryAfterNumericAndDate(t *testing.T) {
	for _, header := range []string{"7", time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)} {
		t.Run(header, func(t *testing.T) {
			attempts := 0
			var delays []time.Duration
			client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				attempts++
				if attempts == 1 {
					w.Header().Set("Retry-After", header)
					w.WriteHeader(429)
					return
				}
				_, _ = w.Write([]byte(`{"data":[]}`))
			}, WithRetry(1, time.Millisecond, 3*time.Second), withSleep(func(ctx context.Context, d time.Duration) error { delays = append(delays, d); return nil }))
			defer server.Close()
			if _, err := client.Regions(context.Background(), PageOptions{}); err != nil {
				t.Fatal(err)
			}
			if len(delays) != 1 || delays[0] < 0 || delays[0] > 3*time.Second {
				t.Fatalf("delays=%v", delays)
			}
		})
	}
}

func TestMalformedResponses(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{`)) }, WithRetry(0, 0, 0))
		defer server.Close()
		if _, err := client.Regions(context.Background(), PageOptions{}); err == nil || !strings.Contains(err.Error(), "decode response") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("error", func(t *testing.T) {
		client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Request-ID", "header-request")
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{`))
		}, WithRetry(0, 0, 0))
		defer server.Close()
		_, err := client.Regions(context.Background(), PageOptions{})
		var apiErr *Error
		if !errors.As(err, &apiErr) || apiErr.Code != "HTTP_ERROR" || apiErr.Status != 500 || apiErr.RequestID != "header-request" {
			t.Fatalf("error=%#v", err)
		}
	})
	t.Run("missing request id", func(t *testing.T) {
		client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Request-ID", "fallback-id")
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"missing"}}`))
		}, WithRetry(0, 0, 0))
		defer server.Close()
		_, err := client.Region(context.Background(), "missing")
		var apiErr *Error
		if !errors.As(err, &apiErr) || apiErr.RequestID != "fallback-id" {
			t.Fatalf("error=%#v", err)
		}
	})
}

func TestTelemetryReportsAccurateErrorCode(t *testing.T) {
	var events []TelemetryEvent
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"missing"}}`))
	}, WithRetry(0, 0, 0), WithTelemetry(func(event TelemetryEvent) { events = append(events, event) }))
	defer server.Close()
	_, _ = client.Region(context.Background(), "missing")
	if len(events) != 1 || events[0].ErrorCode != "NOT_FOUND" {
		t.Fatalf("events=%#v", events)
	}
}

func TestBoundedAtomicDownloadsAndChecksum(t *testing.T) {
	payload := []byte("fixture-download")
	sum := sha256.Sum256(payload)
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(payload) }, WithRetry(0, 0, 0))
	defer server.Close()
	directory := t.TempDir()
	destination := filepath.Join(directory, "artifact.json")
	if err := client.DownloadDatasetArtifactTo(context.Background(), "2026.08.1", "regions", "json", destination, DownloadOptions{MaxBytes: 1024, SHA256: fmt.Sprintf("%x", sum)}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(destination)
	if !reflect.DeepEqual(got, payload) {
		t.Fatalf("payload=%q", got)
	}
	if err := client.DownloadDatasetArtifactTo(context.Background(), "2026.08.1", "regions", "json", destination, DownloadOptions{MaxBytes: 4}); !errors.Is(err, ErrDownloadTooLarge) {
		t.Fatalf("size error=%v", err)
	}
	got, _ = os.ReadFile(destination)
	if !reflect.DeepEqual(got, payload) {
		t.Fatal("failed download replaced destination")
	}
	if err := client.DownloadDatasetArtifactTo(context.Background(), "2026.08.1", "regions", "json", destination, DownloadOptions{SHA256: strings.Repeat("0", 64)}); !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("checksum error=%v", err)
	}
}

func TestAllCanonicalErrorSentinels(t *testing.T) {
	sentinels := []*Error{ErrInvalidArgument, ErrInvalidCoordinates, ErrRadiusOutOfRange, ErrQueryTooShort, ErrPayloadTooLarge, ErrUnauthenticated, ErrKeyRevoked, ErrPermissionDenied, ErrOriginNotAllowed, ErrNotFound, ErrResourceGone, ErrRateLimited, ErrQuotaExceeded, ErrQueryTooComplex, ErrDeadlineExceeded, ErrInternal}
	if len(sentinels) != 16 {
		t.Fatal(len(sentinels))
	}
	for _, sentinel := range sentinels {
		if !errors.Is(&Error{Code: sentinel.Code}, sentinel) {
			t.Fatalf("sentinel %s", sentinel.Code)
		}
	}
}

func TestAnonymousRequestAndTelemetryOptOut(t *testing.T) {
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("unexpected authorization %q", got)
		}
		if got := r.Header.Get("X-GhanaGeo-SDK"); got != "" {
			t.Errorf("unexpected telemetry %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"datasetVersion":"2026.08.3-ulid"}`))
	})
	defer server.Close()
	page, err := client.Regions(context.Background(), PageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if page.DatasetVersion != "2026.08.3-ulid" {
		t.Fatalf("dataset=%q", page.DatasetVersion)
	}
}

func TestTypedErrorAndErrorsIsAs(t *testing.T) {
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"missing","requestId":"req-1","details":{"kind":"region"}}}`))
	})
	defer server.Close()
	_, err := client.Region(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("errors.Is: %v", err)
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != 404 || apiErr.RequestID != "req-1" {
		t.Fatalf("typed error: %#v", apiErr)
	}
}

func TestRetriesSafeReadAndCapsConfiguration(t *testing.T) {
	attempts := 0
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(503)
			return
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}, WithRetry(99, time.Millisecond, time.Second))
	defer server.Close()
	if _, err := client.Regions(context.Background(), PageOptions{}); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("attempts=%d", attempts)
	}
	if client.retries != 5 {
		t.Fatalf("retry cap=%d", client.retries)
	}
}

func TestCancellationStopsRetryDelay(t *testing.T) {
	started := make(chan struct{})
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }, withSleep(func(ctx context.Context, d time.Duration) error { close(started); <-ctx.Done(); return ctx.Err() }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := client.Regions(ctx, PageOptions{}); done <- err }()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestPageIteratorIsLazyAndRejectsRepeatedCursor(t *testing.T) {
	calls := 0
	iterator := NewPageIterator(func(ctx context.Context, cursor string) (Page[int], error) {
		calls++
		return Page[int]{Data: []int{calls}, NextCursor: "same"}, nil
	})
	if calls != 0 {
		t.Fatal("iterator fetched eagerly")
	}
	if !iterator.Next(context.Background()) || !reflect.DeepEqual(iterator.Page().Data, []int{1}) {
		t.Fatal("first page")
	}
	if iterator.Next(context.Background()) {
		t.Fatal("expected repeated cursor failure")
	}
	if !errors.Is(iterator.Err(), ErrRepeatedCursor) {
		t.Fatalf("error=%v", iterator.Err())
	}
}

func TestAllPublicPaths(t *testing.T) {
	var paths []string
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.EscapedPath())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"downloads":[],"geometry":{},"properties":{},"type":"Feature","datasetVersion":"x"}`))
	}, WithRetry(0, 0, 0))
	defer server.Close()
	ctx := context.Background()
	_, _ = client.Regions(ctx, PageOptions{})
	_, _ = client.Region(ctx, "a/b")
	_, _ = client.RegionDistricts(ctx, "r", PageOptions{})
	_, _ = client.Districts(ctx, DistrictOptions{})
	_, _ = client.District(ctx, "d")
	_, _ = client.DistrictPlaces(ctx, "d", PlaceOptions{})
	_, _ = client.Places(ctx, PlaceOptions{})
	_, _ = client.Place(ctx, "p")
	_, _ = client.Search(ctx, "osu", SearchOptions{})
	_, _ = client.Autocomplete(ctx, "os", 10)
	_, _ = client.Geocode(ctx, "osu", 10)
	_, _ = client.ReverseGeocode(ctx, 5.5, -.2)
	_, _ = client.Nearby(ctx, 5.5, -.2, 5000, 20)
	_, _ = client.Boundary(ctx, "r")
	_, _ = client.Datasets(ctx)
	_, _ = client.DatasetDownloads(ctx, "2026.08.3-ulid")
	_, _ = client.Roads(ctx)
	_, _ = client.PointsOfInterest(ctx)
	want := []string{"/regions", "/regions/a%2Fb", "/regions/r/districts", "/districts", "/districts/d", "/districts/d/places", "/places", "/places/p", "/search", "/autocomplete", "/geocode", "/reverse", "/nearby", "/boundaries/r", "/datasets", "/datasets/2026.08.3-ulid/downloads", "/roads", "/pois"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths\n got %#v\nwant %#v", paths, want)
	}
}
