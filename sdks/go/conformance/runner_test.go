package main

import "testing"

func TestInputFailuresRejectOmittedAndMangledParameters(t *testing.T) {
	caseDef := contractCase{ID: "test", Input: map[string]any{"query": map[string]any{"q": "Kumasi", "limit": float64(2)}, "path": map[string]any{"id": "gh-place-kumasi"}}}
	valid := []requestTrace{{Method: "GET", Path: "/v1/places/gh-place-kumasi", Query: "limit=2&q=Kumasi"}}
	if failures := inputFailures(caseDef, valid); len(failures) != 0 {
		t.Fatalf("valid trace: %v", failures)
	}
	for name, trace := range map[string][]requestTrace{"omitted": {{Method: "GET", Path: "/v1/places/gh-place-kumasi", Query: "q=Kumasi"}}, "mangled": {{Method: "GET", Path: "/v1/places/wrong", Query: "limit=9&q=Kumasi"}}} {
		t.Run(name, func(t *testing.T) {
			if failures := inputFailures(caseDef, trace); len(failures) == 0 {
				t.Fatal("expected mapping failure")
			}
		})
	}
}
