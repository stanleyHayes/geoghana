// Package relevance scores how well a candidate matches a query.
//
// WHY THIS EXISTS AND IS NOT LEFT TO THE SEARCH ENGINE:
//
// Spec section 10 requires every result to carry a confidence score and a match
// explanation, and requires ambiguous queries to return several candidates
// rather than a guess. A search engine's internal score cannot serve that.
// Typesense packs typo distance, field weight and token count into one opaque
// int64, and for the query "acra" it returns the SAME value for "Accra" and for
// "Kwahu Afram Plains South" — both matched one token at the same typo
// distance. Normalising that to 0..1 yields 1.0 for everything, which would
// make the admin's low-confidence-query queue meaningless.
//
// Scoring here also means Typesense and Atlas Search produce identical
// confidences, which is the whole point of the SearchPort abstraction.
package relevance

import (
	"math"
	"strings"
)

// MatchKind describes how a candidate matched, in decreasing strength.
type MatchKind int

const (
	NoMatch MatchKind = iota
	FuzzyMatch
	TokenMatch
	PrefixMatch
	AliasMatch
	ExactMatch
)

func (k MatchKind) String() string {
	switch k {
	case ExactMatch:
		return "exact match"
	case AliasMatch:
		return "alias match"
	case PrefixMatch:
		return "prefix match"
	case TokenMatch:
		return "token match"
	case FuzzyMatch:
		return "fuzzy match"
	default:
		return "no match"
	}
}

// Result is a score in 0..1 plus a human explanation of why it matched.
type Result struct {
	Score  float64
	Kind   MatchKind
	Reason string
}

// Score compares a normalized query against a normalized candidate name and
// its normalized aliases. All inputs must already have been through
// normalize.Name, so this function is purely about similarity.
func Score(query, name string, aliases []string) Result {
	query = strings.TrimSpace(query)
	name = strings.TrimSpace(name)
	if query == "" || name == "" {
		return Result{Kind: NoMatch, Reason: "no match"}
	}

	best := scoreOne(query, name)
	best.Reason = best.Kind.String() + ": " + name

	for _, a := range aliases {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		r := scoreOne(query, a)
		// An alias hit is worth slightly less than the same hit on the
		// canonical name: the canonical name is what we are most sure of.
		r.Score *= 0.95
		if r.Kind == ExactMatch {
			r.Kind = AliasMatch
		}
		if r.Score > best.Score {
			r.Reason = "alias match: " + a
			best = r
		}
	}
	return best
}

func scoreOne(query, candidate string) Result {
	if query == candidate {
		return Result{Score: 1, Kind: ExactMatch}
	}

	// Prefix: "accra metro" against "accra metropolitan". Confidence scales
	// with how much of the candidate the query actually covers, so a two-letter
	// prefix of a long name scores low even though it technically matches.
	if strings.HasPrefix(candidate, query) {
		coverage := float64(len(query)) / float64(len(candidate))
		return Result{Score: 0.80 + 0.19*coverage, Kind: PrefixMatch}
	}

	qTokens := strings.Fields(query)
	cTokens := strings.Fields(candidate)

	// Every query token present as a candidate token, in any order.
	matched := 0
	for _, qt := range qTokens {
		for _, ct := range cTokens {
			if qt == ct || strings.HasPrefix(ct, qt) {
				matched++
				break
			}
		}
	}
	if matched == len(qTokens) && matched > 0 {
		coverage := float64(matched) / float64(len(cTokens))
		return Result{Score: 0.65 + 0.20*coverage, Kind: TokenMatch}
	}

	// Fuzzy: edit distance over the whole string, then over the best token
	// pair, whichever is kinder. Similarity below 0.5 is not a match at all.
	sim := similarity(query, candidate)
	matchedLen := len(candidate)
	for _, qt := range qTokens {
		for _, ct := range cTokens {
			if s := similarity(qt, ct); s > sim {
				sim = s
				matchedLen = len(ct)
			}
		}
	}
	if sim < 0.5 {
		return Result{Score: 0, Kind: NoMatch}
	}

	// Map similarity 0.5..1.0 onto 0.25..0.85.
	base := 0.25 + 0.60*((sim-0.5)/0.5)

	// Then weight by COVERAGE: how much of the candidate the matched portion
	// accounts for. This is what separates "accra" from "greater accra" for
	// the query "acra" — both contain an equally-good fuzzy token, but in one
	// the query is the whole name and in the other it is half of it. Without
	// this the two tie, which is the defect that motivated this package.
	coverage := float64(matchedLen) / float64(len(candidate))
	score := base * (0.7 + 0.3*coverage)

	// A fuzzy hit must never reach the token-match floor of 0.65.
	return Result{Score: math.Min(score, 0.64), Kind: FuzzyMatch}
}

// similarity is 1 - normalized Levenshtein distance.
func similarity(a, b string) float64 {
	if a == b {
		return 1
	}
	if a == "" || b == "" {
		return 0
	}
	d := levenshtein([]rune(a), []rune(b))
	longest := max(len([]rune(a)), len([]rune(b)))
	return 1 - float64(d)/float64(longest)
}

// levenshtein computes edit distance with two rolling rows.
func levenshtein(a, b []rune) int {
	if len(a) < len(b) {
		a, b = b, a
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(min(curr[j-1]+1, prev[j]+1), prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}
