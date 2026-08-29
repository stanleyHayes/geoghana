package relevance

import "testing"

// The bug this package exists to fix: Typesense returned an identical score for
// "Accra" and "Kwahu Afram Plains South" on the query "acra", so normalising it
// gave 1.0 to both. A confidence score that cannot separate a right answer from
// a wrong one is worse than none, because the admin's low-confidence queue and
// every caller's threshold depend on it.
func TestRealAnswerOutranksIncidentalFuzzyMatch(t *testing.T) {
	right := Score("acra", "accra", nil)
	wrong := Score("acra", "kwahu afram plains south", nil)

	if right.Score <= wrong.Score {
		t.Fatalf("'accra' scored %.2f but 'kwahu afram plains south' scored %.2f for query 'acra'",
			right.Score, wrong.Score)
	}
	if wrong.Score > 0.5 {
		t.Errorf("an incidental fuzzy hit scored %.2f; it should be clearly low", wrong.Score)
	}
}

func TestExactMatchIsOne(t *testing.T) {
	r := Score("kumasi", "kumasi", nil)
	if r.Score != 1 || r.Kind != ExactMatch {
		t.Fatalf("got %.2f / %v, want 1.00 / ExactMatch", r.Score, r.Kind)
	}
}

func TestPrefixScoresByCoverage(t *testing.T) {
	// A long prefix of a name is more confident than a short one.
	long := Score("accra metropolita", "accra metropolitan", nil)
	short := Score("ac", "accra metropolitan", nil)
	if long.Score <= short.Score {
		t.Errorf("long prefix %.2f should beat short prefix %.2f", long.Score, short.Score)
	}
	if short.Score >= 0.9 {
		t.Errorf("a two-letter prefix scored %.2f; that is overconfident", short.Score)
	}
}

func TestAllTokensPresentBeatsPartial(t *testing.T) {
	full := Score("accra metropolitan", "accra metropolitan", nil)
	partial := Score("accra", "accra metropolitan", nil)
	if full.Score <= partial.Score {
		t.Errorf("full match %.2f should beat partial %.2f", full.Score, partial.Score)
	}
}

func TestAliasMatchIsReported(t *testing.T) {
	r := Score("osu re", "osu", []string{"osu re"})
	if r.Kind != AliasMatch {
		t.Errorf("kind = %v, want AliasMatch", r.Kind)
	}
	if r.Reason != "alias match: osu re" {
		t.Errorf("reason = %q, want the alias named", r.Reason)
	}
	// An alias hit is worth slightly less than the same hit on the canonical name.
	if r.Score >= 1.0 {
		t.Errorf("alias score %.2f should be below a canonical exact match", r.Score)
	}
}

func TestOrderingIsSaneAcrossKinds(t *testing.T) {
	exact := Score("tema", "tema", nil).Score
	prefix := Score("tema", "tema metropolitan", nil).Score
	fuzzy := Score("tena", "tema", nil).Score
	none := Score("zzzz", "tema", nil).Score

	if !(exact > prefix && prefix > fuzzy && fuzzy > none) {
		t.Errorf("expected exact > prefix > fuzzy > none, got %.2f %.2f %.2f %.2f",
			exact, prefix, fuzzy, none)
	}
	if none != 0 {
		t.Errorf("a non-match scored %.2f, want 0", none)
	}
}

func TestScoreIsAlwaysInRange(t *testing.T) {
	cases := [][2]string{
		{"a", "accra"}, {"accra", "a"}, {"", "accra"}, {"accra", ""},
		{"kwabenya", "kwabenya"}, {"tema community 25", "tema metropolitan"},
		{"xyz", "greater accra"},
	}
	for _, c := range cases {
		r := Score(c[0], c[1], nil)
		if r.Score < 0 || r.Score > 1 {
			t.Errorf("Score(%q, %q) = %.3f, out of range", c[0], c[1], r.Score)
		}
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"kumasi", "kumsai", 2}, // transposition costs two edits
		{"acra", "accra", 1},
		{"", "abc", 3},
		{"same", "same", 0},
	}
	for _, c := range cases {
		if got := levenshtein([]rune(c.a), []rune(c.b)); got != c.want {
			t.Errorf("levenshtein(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// The exact defect that motivated coverage weighting: for the query "acra",
// "accra" (the query is effectively the whole name) must beat "greater accra"
// (the query covers only part of it), even though both contain an equally
// good fuzzy token.
func TestCoverageSeparatesWholeNameFromPartialName(t *testing.T) {
	whole := Score("acra", "accra", nil)
	partial := Score("acra", "greater accra", nil)
	incidental := Score("acra", "kwahu afram plains south", nil)

	if !(whole.Score > partial.Score && partial.Score > incidental.Score) {
		t.Fatalf("expected accra(%.2f) > greater accra(%.2f) > kwahu(%.2f)",
			whole.Score, partial.Score, incidental.Score)
	}
	// A single-character typo on a short name should read as a decent match,
	// not a marginal one.
	if whole.Score < 0.5 {
		t.Errorf("a one-edit typo scored %.2f; that is over-pessimistic", whole.Score)
	}
}

func TestFuzzyNeverOutranksTokenMatch(t *testing.T) {
	fuzzy := Score("acra", "accra", nil)
	token := Score("accra", "accra metropolitan", nil)
	if fuzzy.Score >= token.Score {
		t.Errorf("fuzzy %.2f must stay below a real token/prefix match %.2f",
			fuzzy.Score, token.Score)
	}
}
