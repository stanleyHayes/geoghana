package ingest

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/normalize"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/relevance"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

// AutoMatchThreshold is how confident a fuzzy name match must be before a
// boundary is attached without a human looking at it.
//
// Set high on purpose. Ghana's district names carry real transliteration
// variance — Akwapem/Akuapim, Afadzato/Afadjato — and a wrong boundary is
// worse than a missing one: it would silently misroute every containment
// query for that district. Anything below this becomes a review item.
const AutoMatchThreshold = 0.88

// BoundaryMatch records one attempted match, matched or not, so a steward can
// see the whole picture rather than only the failures.
type BoundaryMatch struct {
	SourceName string
	TargetID   string
	TargetName string
	Score      float64
	Exact      bool
	// Runner-up, when there was one. Two close candidates mean the match is
	// genuinely ambiguous and must not be applied automatically.
	RunnerUpName  string
	RunnerUpScore float64
	Reason        string
}

// BoundaryResult summarises an import for the import-runs screen.
type BoundaryResult struct {
	Level          string
	Total          int
	Exact          int
	Fuzzy          int
	Unmatched      []BoundaryMatch
	Ambiguous      []BoundaryMatch
	InvalidGeom    []string
	AppliedFuzzy   []BoundaryMatch
	TargetsWithout []string
}

func (r BoundaryResult) String() string {
	return fmt.Sprintf("%s: %d features · %d exact · %d fuzzy applied · %d ambiguous · %d unmatched · %d invalid geometry",
		r.Level, r.Total, r.Exact, r.Fuzzy, len(r.Ambiguous), len(r.Unmatched), len(r.InvalidGeom))
}

// nameCandidate is one canonical record a boundary might belong to.
type NameCandidate struct {
	ID         string
	Name       string
	Normalized string
	// RegionID scopes a district candidate to its region, so a name match can
	// be gated on geography rather than trusting the name alone.
	RegionID string
}

// MatchByName resolves a boundary's name to a canonical record.
//
// Three outcomes, and the distinction between the last two is the point:
//   - exact normalized match      → apply
//   - one clear high-scoring hit  → apply, flagged for review
//   - anything else               → do NOT apply; report for a steward
//
// Rule R8: no source overwrites canonical data merely because it arrived. A
// boundary attached to the wrong district is invisible until someone notices
// their reverse geocode has been wrong for months.
func MatchByName(sourceName string, candidates []NameCandidate) BoundaryMatch {
	ns := canonicalKey(sourceName)

	for _, c := range candidates {
		if c.Normalized == ns {
			return BoundaryMatch{
				SourceName: sourceName, TargetID: c.ID, TargetName: c.Name,
				Score: 1, Exact: true, Reason: "exact name match",
			}
		}
	}

	type scored struct {
		c NameCandidate
		s float64
	}
	all := make([]scored, 0, len(candidates))
	for _, c := range candidates {
		// NameSimilarity, not Score: this is name identity, not search ranking.
		all = append(all, scored{c, relevance.NameSimilarity(ns, c.Normalized)})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].s > all[j].s })

	if len(all) == 0 {
		return BoundaryMatch{SourceName: sourceName, Reason: "no candidates"}
	}

	best := all[0]
	m := BoundaryMatch{
		SourceName: sourceName, TargetID: best.c.ID, TargetName: best.c.Name, Score: best.s,
	}
	if len(all) > 1 {
		m.RunnerUpName, m.RunnerUpScore = all[1].c.Name, all[1].s
	}

	switch {
	case best.s < AutoMatchThreshold:
		m.TargetID = ""
		m.Reason = fmt.Sprintf("best candidate %q scored %.2f, below the %.2f threshold",
			best.c.Name, best.s, AutoMatchThreshold)
	case m.RunnerUpScore >= AutoMatchThreshold:
		// Two plausible districts is exactly the case Spec §4.2 says never to
		// resolve automatically.
		m.TargetID = ""
		m.Reason = fmt.Sprintf("ambiguous: %q (%.2f) and %q (%.2f) both plausible",
			best.c.Name, best.s, m.RunnerUpName, m.RunnerUpScore)
	default:
		m.Reason = fmt.Sprintf("fuzzy match at %.2f", best.s)
	}
	return m
}

// canonicalKey reduces a name to what actually identifies the place.
//
// It MUST be applied to both sides of a comparison. An earlier version stripped
// administrative words from the source name only, so "Mampong Municipal" was
// compared against "mampong municipal" and two identical names scored 0.88 —
// asymmetric normalization is a silent, self-inflicted mismatch.
//
// The words removed differ purely by source convention: geoBoundaries writes
// "Adenta Municipal" and "Western North Region" where our seed writes "Adenta"
// and "Western North". They carry no identifying information, and Ghana has
// districts whose only difference IS the classifier, so they are removed
// consistently rather than weighted.
func canonicalKey(s string) string {
	n := normalize.Name(s)
	drop := map[string]bool{
		"region": true, "metropolis": true, "metropolitan": true,
		"municipality": true, "municipal": true, "district": true, "assembly": true,
	}
	var kept []string
	for _, tok := range strings.Fields(n) {
		if !drop[tok] {
			kept = append(kept, tok)
		}
	}
	// Never reduce a name to nothing: if every token was a classifier, the
	// original is the only thing we have.
	if len(kept) == 0 {
		return n
	}
	return strings.Join(kept, " ")
}

// RegionCandidates and DistrictCandidates build the lookup sets.

func RegionCandidates(ctx context.Context, repo ports.RegionRepository) ([]NameCandidate, error) {
	page, err := repo.List(ctx, ports.ListParams{Limit: 100})
	if err != nil {
		return nil, err
	}
	out := make([]NameCandidate, 0, len(page.Data))
	for _, r := range page.Data {
		out = append(out, NameCandidate{ID: r.ID, Name: r.Name, Normalized: canonicalKey(r.Name)})
	}
	return out, nil
}

func DistrictCandidates(ctx context.Context, repo ports.DistrictRepository) ([]NameCandidate, error) {
	var out []NameCandidate
	cursor := ""
	for {
		page, err := repo.List(ctx, ports.DistrictFilter{
			ListParams: ports.ListParams{Limit: 100, Cursor: cursor},
		})
		if err != nil {
			return nil, err
		}
		for _, d := range page.Data {
			out = append(out, NameCandidate{
				ID: d.ID, Name: d.Name, Normalized: canonicalKey(d.Name), RegionID: d.RegionID,
			})
		}
		if page.NextCursor == "" {
			return out, nil
		}
		cursor = page.NextCursor
	}
}

// InRegion narrows candidates to one region.
//
// This is the strongest available guard on a name match, and it is
// geometric rather than textual: a boundary can only belong to a district in
// the region its own shape sits inside. Without it the comparison is national,
// and "Bolgatanga East" (Upper East) scores against "Ga East" (Greater Accra)
// — two districts about 700km apart whose names differ by one word.
//
// It also lets a name variant resolve SAFELY that would be reckless
// nationally: within Ashanti alone, "Akrofuom" and "Adansi Akrofuom" are
// unambiguous.
func InRegion(candidates []NameCandidate, regionID string) []NameCandidate {
	if regionID == "" {
		return candidates
	}
	out := make([]NameCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.RegionID == regionID {
			out = append(out, c)
		}
	}
	return out
}

// BoundaryWriter attaches a validated geometry to a canonical record.
type BoundaryWriter interface {
	SetGeometry(ctx context.Context, id string, g *geography.Geometry) error
}
