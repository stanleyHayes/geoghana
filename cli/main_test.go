package main

import (
	"strings"
	"testing"

	"github.com/ghanageo/ghanageo-cli/internal/render"
)

// Go's flag package stops at the first non-flag argument, so `search accra
// --json` would silently ignore --json without the split in parseFlags.
func TestPositionalArgumentsBeforeFlags(t *testing.T) {
	opts, pos, err := parseFlags([]string{"tema", "community", "--json", "--limit", "5"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := strings.Join(pos, " "); got != "tema community" {
		t.Errorf("positional = %q, want \"tema community\"", got)
	}
	if opts.format != render.FormatJSON {
		t.Errorf("format = %v, want JSON — a flag after a positional was dropped", opts.format)
	}
	if opts.limit != 5 {
		t.Errorf("limit = %d, want 5", opts.limit)
	}
}

func TestFlagsBeforePositionalAlsoWork(t *testing.T) {
	_, pos, err := parseFlags([]string{"--limit", "3", "accra"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if strings.Join(pos, " ") != "accra" {
		t.Errorf("positional = %v, want [accra]", pos)
	}
}

func TestJSONAndCSVAreMutuallyExclusive(t *testing.T) {
	if _, _, err := parseFlags([]string{"--json", "--csv"}); err == nil {
		t.Error("expected an error when both output formats are requested")
	}
}

func TestDefaultsAreSensible(t *testing.T) {
	o, _, err := parseFlags(nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if o.format != render.FormatTable {
		t.Error("default format should be the human table")
	}
	if o.limit != 20 {
		t.Errorf("default limit = %d, want 20", o.limit)
	}
	// The CLI must work with no key: GhanaGeo is free.
	if o.apiKey != "" && strings.TrimSpace(o.apiKey) == "" {
		t.Error("api key should default to empty")
	}
}

func TestParseLatLng(t *testing.T) {
	lat, lng, err := parseLatLng([]string{"5.556", "-0.182"})
	if err != nil || lat != 5.556 || lng != -0.182 {
		t.Fatalf("got %v,%v err=%v", lat, lng, err)
	}
	if _, _, err := parseLatLng([]string{"5.556"}); err == nil {
		t.Error("a single argument should be rejected")
	}
	if _, _, err := parseLatLng([]string{"north", "0"}); err == nil {
		t.Error("a non-numeric latitude should be rejected")
	}
}

func TestDashPlaceholders(t *testing.T) {
	if dash("") != "—" || dash("   ") != "—" {
		t.Error("blank values should render as an em dash, not empty space")
	}
	if dash("Kumasi") != "Kumasi" {
		t.Error("a real value must pass through unchanged")
	}
}
