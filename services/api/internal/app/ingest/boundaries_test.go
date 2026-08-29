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

// Region gating is the strongest guard available on a boundary name match,
// and the reason it exists is concrete: geoBoundaries has no parent-region
// field, so a national comparison let "Bolgatanga East" (Upper East) score
// against "Ga East" (Greater Accra) — two districts 700km apart.
func TestInRegionPreventsCrossRegionMatches(t *testing.T) {
	all := []NameCandidate{
		{ID: "d1", Name: "Ga East", Normalized: "ga east", RegionID: "gh-region-greater-accra"},
		{ID: "d2", Name: "Bolgatanga East", Normalized: "bolgatanga east", RegionID: "gh-region-upper-east"},
		{ID: "d3", Name: "Bolgatanga Municipal", Normalized: "bolgatanga", RegionID: "gh-region-upper-east"},
	}

	upperEast := InRegion(all, "gh-region-upper-east")
	if len(upperEast) != 2 {
		t.Fatalf("expected 2 Upper East candidates, got %d", len(upperEast))
	}
	for _, c := range upperEast {
		if c.Name == "Ga East" {
			t.Error("a Greater Accra district survived an Upper East filter")
		}
	}

	// Matching nationally can reach the wrong region; matching within the
	// region cannot, whatever the score says.
	national := MatchByName("Bolgatanga East", all)
	gated := MatchByName("Bolgatanga East", upperEast)
	if gated.TargetID != "" && gated.TargetName == "Ga East" {
		t.Error("region gating still resolved to Ga East")
	}
	_ = national

	// An empty region id must not silently drop every candidate, or a source
	// whose region could not be resolved would match nothing at all.
	if got := InRegion(all, ""); len(got) != len(all) {
		t.Errorf("InRegion with no region returned %d of %d", len(got), len(all))
	}
}

// Region gating improves RANKING; it does not lower the bar for applying.
//
// "Adansi Akrofuom" is almost certainly our "Akrofuom" — geoBoundaries carries
// the older compound name — but it scores 0.53, well under the auto-apply
// threshold, so it is surfaced for a steward instead of written. That is rule
// R8: a source does not overwrite canonical data merely because it arrived,
// and a boundary attached to the wrong district is invisible until someone
// notices their reverse geocode has been wrong for months.
func TestRegionGatingRanksWithoutLoweringTheBar(t *testing.T) {
	ashanti := []NameCandidate{
		{ID: "d1", Name: "Akrofuom", Normalized: "akrofuom", RegionID: "gh-region-ashanti"},
		{ID: "d2", Name: "Adansi South", Normalized: "adansi south", RegionID: "gh-region-ashanti"},
	}
	m := MatchByName("Adansi Akrofuom", ashanti)

	// The right district ranks first...
	if m.TargetName != "Akrofuom" {
		t.Errorf("best candidate = %q, want Akrofuom", m.TargetName)
	}
	// ...and is still NOT applied automatically.
	if m.TargetID != "" {
		t.Errorf("a 0.53 match was auto-applied: %+v", m)
	}
	if m.Reason == "" {
		t.Error("an unapplied match must explain itself to the steward")
	}
}

// The directional trap CLAUDE.md records: "Atwima Nwabiagya" exists in the
// source as North and South, and our record is the un-split Municipal. Two
// plausible candidates must never be resolved automatically (Spec §4.2).
func TestSplitDistrictIsNeverAutoResolved(t *testing.T) {
	ashanti := []NameCandidate{
		{ID: "n", Name: "Atwima Nwabiagya North", Normalized: "atwima nwabiagya north", RegionID: "gh-region-ashanti"},
		{ID: "s", Name: "Atwima Nwabiagya South", Normalized: "atwima nwabiagya south", RegionID: "gh-region-ashanti"},
	}
	m := MatchByName("Atwima Nwabiagya Municipal", ashanti)
	if m.TargetID != "" {
		t.Errorf("a split district was auto-resolved to %q — the other half would "+
			"have been silently wrong: %+v", m.TargetName, m)
	}
}
