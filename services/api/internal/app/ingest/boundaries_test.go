package ingest

import "testing"

// Regression: canonicalKey must be applied to BOTH sides. Stripping
// administrative words from only the source name made two identical names
// score 0.88 — "Mampong Municipal" vs "Mampong Municipal" failed to match.
func TestIdenticalNamesMatchExactly(t *testing.T) {
	for _, name := range []string{
		"Mampong Municipal", "Cape Coast Metropolitan", "Kumasi Metropolitan",
		"Tema West Municipal", "Effutu Municipal",
	} {
		c := []NameCandidate{{ID: "x", Name: name, Normalized: canonicalKey(name)}}
		m := MatchByName(name, c)
		if !m.Exact || m.Score != 1 {
			t.Errorf("%q vs itself: exact=%v score=%.2f — identical names must match exactly",
				name, m.Exact, m.Score)
		}
	}
}

// Classifiers differ purely by source convention and carry no identifying
// information, so they must not prevent a match.
func TestClassifiersDoNotBlockAMatch(t *testing.T) {
	cases := [][2]string{
		{"Adenta Municipal", "Adenta"},
		{"Western North Region", "Western North"},
		{"Kumbungu", "Kumbungu District"},
		{"Yendi Municipal", "Yendi Municipal District"},
		{"Obuasi East", "Obuasi East Municipal"},
	}
	for _, c := range cases {
		cand := []NameCandidate{{ID: "x", Name: c[1], Normalized: canonicalKey(c[1])}}
		m := MatchByName(c[0], cand)
		if m.TargetID == "" {
			t.Errorf("%q → %q was not matched: %s", c[0], c[1], m.Reason)
		}
	}
}

// A genuinely different district must NOT be matched. Attaching the wrong
// boundary silently misroutes every containment query for it, which is worse
// than leaving it unmatched for a steward.
func TestGenuinelyDifferentNamesDoNotMatch(t *testing.T) {
	candidates := []NameCandidate{
		{ID: "a", Name: "Assin North", Normalized: canonicalKey("Assin North")},
		{ID: "b", Name: "Tamale Metropolitan", Normalized: canonicalKey("Tamale Metropolitan")},
		{ID: "c", Name: "North Gonja", Normalized: canonicalKey("North Gonja")},
	}
	for _, source := range []string{"Assin Fosu", "Tatale Sanguli", "Akwapem North"} {
		m := MatchByName(source, candidates)
		if m.TargetID != "" {
			t.Errorf("%q was matched to %q at %.2f — it should have gone to review",
				source, m.TargetName, m.Score)
		}
	}
}

// Two plausible candidates is exactly the case Spec §4.2 says never to resolve
// automatically.
func TestAmbiguousMatchesAreRefused(t *testing.T) {
	candidates := []NameCandidate{
		{ID: "a", Name: "Adansi North", Normalized: canonicalKey("Adansi North")},
		{ID: "b", Name: "Adansi South", Normalized: canonicalKey("Adansi South")},
	}
	m := MatchByName("Adansi Nort", candidates)
	if m.TargetID != "" && m.RunnerUpScore >= AutoMatchThreshold {
		t.Errorf("an ambiguous match was applied: %s", m.Reason)
	}
}

func TestCanonicalKeyNeverEmpties(t *testing.T) {
	// A name made entirely of classifiers must not reduce to nothing.
	if got := canonicalKey("Municipal District"); got == "" {
		t.Error("canonicalKey reduced a name to the empty string")
	}
}
