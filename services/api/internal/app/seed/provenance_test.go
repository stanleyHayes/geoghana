package seed

import (
	"strings"
	"testing"
)

func TestSeedProvenanceIsCompleteAndDeterministic(t *testing.T) {
	row := map[string]string{
		"id": "gh-region-ahafo", "name": "Ahafo",
		"source_url": "https://example.test/source", "retrieved_at": "2026-08-28",
	}
	first := provenanceFrom(row, "seed-bootstrap")
	second := provenanceFrom(row, "seed-bootstrap")
	if first.ExternalID != row["id"] || first.RetrievedAt == "" {
		t.Fatalf("source identity is incomplete: %+v", first)
	}
	if first.SourcePayloadHash != second.SourcePayloadHash || len(first.SourcePayloadHash) != 64 {
		t.Fatalf("payload hash is not stable SHA-256: %q / %q", first.SourcePayloadHash, second.SourcePayloadHash)
	}
	if strings.Trim(first.SourcePayloadHash, "0123456789abcdef") != "" {
		t.Fatalf("payload hash is not lowercase hexadecimal: %q", first.SourcePayloadHash)
	}
	row["name"] = "Ahafo changed"
	if changed := provenanceFrom(row, "seed-bootstrap"); changed.SourcePayloadHash == first.SourcePayloadHash {
		t.Fatal("payload changes did not change the provenance hash")
	}
}
