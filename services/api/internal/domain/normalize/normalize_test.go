package normalize

import "testing"

func TestDeterministic(t *testing.T) {
	// Spec 22.1: aliases must normalize deterministically.
	for i := 0; i < 100; i++ {
		if Name("Tema Community 25") != "tema community 25" {
			t.Fatal("normalization is not deterministic")
		}
	}
}

func TestGhanaianOrthographyFolds(t *testing.T) {
	cases := map[string]string{
		"Ɔsu":      "osu",
		"Kwabɛnya": "kwabenya",
		"Ŋmaŋ":     "nman",
		"Ɛdena":    "edena",
		"Aŋlɔga":   "anloga",
	}
	for in, want := range cases {
		if got := Name(in); got != want {
			t.Errorf("Name(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAbbreviationExpansion(t *testing.T) {
	cases := map[string]string{
		"tema comm 25":  "tema community 25",
		"Tema Comm. 25": "tema community 25",
		"Spintex Rd":    "spintex road",
		"Achimota Jn":   "achimota junction",
	}
	for in, want := range cases {
		if got := Name(in); got != want {
			t.Errorf("Name(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPunctuationAndCaseFolding(t *testing.T) {
	variants := []string{"Cape-Coast", "cape coast", "CAPE  COAST", " Cape,Coast "}
	want := "cape coast"
	for _, v := range variants {
		if got := Name(v); got != want {
			t.Errorf("Name(%q) = %q, want %q", v, got, want)
		}
	}
}

func TestEmptyAndWhitespace(t *testing.T) {
	for _, in := range []string{"", "   ", "\t\n"} {
		if got := Name(in); got != "" {
			t.Errorf("Name(%q) = %q, want empty", in, got)
		}
	}
}
