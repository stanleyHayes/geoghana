package mongo

import "testing"

func TestHealthThresholds(t *testing.T) {
	for _, tc := range []struct {
		value float64
		want  string
	}{{0, "healthy"}, {99, "healthy"}, {100, "degraded"}, {499, "degraded"}, {500, "unhealthy"}} {
		if got := thresholdStatus(tc.value, 100, 500); got != tc.want {
			t.Fatalf("thresholdStatus(%v) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestCacheHitRateThresholds(t *testing.T) {
	for _, tc := range []struct {
		value float64
		want  string
	}{{95, "healthy"}, {80, "healthy"}, {79, "degraded"}, {60, "degraded"}, {59, "unhealthy"}} {
		if got := rateMetric(tc.value).Status; got != tc.want {
			t.Fatalf("rateMetric(%v) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestWorseHealthStatus(t *testing.T) {
	if got := worseStatus("degraded", "unavailable"); got != "unavailable" {
		t.Fatalf("worseStatus = %q, want unavailable", got)
	}
	if got := worseStatus("unhealthy", "healthy"); got != "unhealthy" {
		t.Fatalf("worseStatus regressed unhealthy to %q", got)
	}
}
