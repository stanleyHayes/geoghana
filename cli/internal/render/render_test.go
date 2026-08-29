package render

import (
	"bytes"
	"strings"
	"testing"
)

// Column alignment must be measured in RUNES. Ghanaian place names carry ɔ, ɛ
// and ŋ, which are multi-byte; measuring bytes would misalign every table that
// contains one.
func TestTableAlignsByRunesNotBytes(t *testing.T) {
	var b bytes.Buffer
	err := WriteTable(&b, Table{
		Headers: []string{"name", "region"},
		Rows: [][]string{
			{"Ɔsu", "Greater Accra"},
			{"Kwabɛnya", "Greater Accra"},
			{"Accra", "Greater Accra"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	// Every row's second column must start at the same rune offset.
	var offsets []int
	for _, l := range lines {
		idx := strings.Index(l, "Greater Accra")
		if idx < 0 {
			idx = strings.Index(l, "REGION")
		}
		if idx >= 0 {
			offsets = append(offsets, len([]rune(l[:idx])))
		}
	}
	if len(offsets) < 3 {
		t.Fatalf("expected the region column on every row, got %d", len(offsets))
	}
	for i, o := range offsets {
		if o != offsets[0] {
			t.Errorf("row %d starts the region column at rune %d, first row at %d — multi-byte names broke alignment",
				i, o, offsets[0])
		}
	}
}

func TestCSVRoundTrips(t *testing.T) {
	var b bytes.Buffer
	err := WriteCSV(&b, Table{
		Headers: []string{"id", "name"},
		Rows:    [][]string{{"gh-place-accra", "Accra, Greater"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// A value containing a comma must be quoted, or the CSV is corrupt.
	if !strings.Contains(b.String(), `"Accra, Greater"`) {
		t.Errorf("comma in a value was not quoted: %q", b.String())
	}
}

func TestStatusCarriesANonColourGlyph(t *testing.T) {
	// Colour is never the sole carrier of meaning, including in a terminal.
	for status, want := range map[string]string{
		"CANONICAL":                           "✓",
		"REVIEWED":                            "◆",
		"SEED_NEEDS_CANONICAL_RECONCILIATION": "!",
		"REFERENCE":                           "·",
	} {
		if got := Status(status); !strings.Contains(got, want) {
			t.Errorf("Status(%q) = %q, expected a %q glyph", status, got, want)
		}
	}
	if Status("") != "" {
		t.Error("an empty status should render as empty")
	}
}

func TestColourDisabledWhenNotATerminal(t *testing.T) {
	var b bytes.Buffer
	if ColourEnabled(&b) {
		t.Error("colour must be off for a non-terminal writer, or piping injects escape codes")
	}
}
