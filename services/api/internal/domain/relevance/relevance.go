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
	"sort"
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

	// PARTIAL token overlap. How many query tokens matched is real signal, and
	// an earlier version threw it away by falling straight through to
	// whole-string fuzzy. The consequence was visible: for "tema community 25",
	// "tema new town" (one token in common) outranked "tema community 12" (two),
	// because the shared "tema" dominated the fuzzy score and the shorter name
	// then won on coverage.
	if matched > 0 && len(qTokens) > 1 {
		share := float64(matched) / float64(len(qTokens))
		// Reward covering more of the CANDIDATE too, so a two-token hit on a
		// two-token name beats the same hit buried in a long name.
		coverage := float64(matched) / float64(len(cTokens))
		return Result{Score: 0.30 + 0.30*share + 0.04*coverage, Kind: TokenMatch}
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

	// Quality combines closeness with COVERAGE: how much of the candidate the
	// matched portion accounts for. Coverage is what separates "accra" from
	// "greater accra" for the query "acra" — both contain an equally good
	// fuzzy token, but in one the query is the whole name and in the other it
	// is half of it.
	coverage := float64(matchedLen) / float64(len(candidate))
	quality := ((sim - 0.5) / 0.5) * (0.7 + 0.3*coverage) // 0..1

	// Map the band into [0.22, 0.63] by SCALING, not clipping. An earlier
	// version clamped with math.Min, which pushed every decent fuzzy match
	// onto the ceiling: "tema comm 25" returned four places all scoring
	// exactly 0.64, so the ordering carried no information.
	//
	// The exponent below curves the response so near-misses rise quickly.
	// Edit distance is unkind to short words — one wrong letter in "accra" is
	// 20% of it — and a linear map made a single typo read as a weak match.
	// Fuzzy still stays strictly under the token-match floor of 0.65.
	return Result{Score: 0.22 + 0.41*math.Pow(quality, 0.6), Kind: FuzzyMatch}
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

// NameSimilarity measures how likely two administrative names refer to the
// SAME place, in 0..1.
//
// This is a different question from Score, which ranks search results. Search
// asks "how well does this candidate answer the user's query"; reconciliation
// asks "are these two strings the same name written differently". Using the
// search scorer for reconciliation produced nonsense: "kasena nankana west"
// and "kassena nankana west" — one letter apart — scored 0.53, because the
// search scorer picks the best-matching TOKEN pair and a perfect "west"/"west"
// hit then scored badly on coverage.
//
// Two measures are combined, because Ghanaian administrative names vary in two
// distinct ways:
//
//   - SPELLING: Mfantseman/Mfantsiman, Sagnerigu/Sagnarigu, Kasena/Kassena.
//     Whole-string edit distance catches these.
//   - WORD ORDER: "Asene Akroso Manso" and "Asene Manso Akroso" are the same
//     district. Comparing sorted token sets catches these, and edit distance
//     alone never would.
func NameSimilarity(a, b string) float64 {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}

	// DIRECTIONAL GUARD, and this one matters more than the rest of the
	// function. Ghana has many district pairs distinguished ONLY by a compass
	// word: Atwima Nwabiagya North and Atwima Nwabiagya South, Awutu Senya
	// East and West, Assin North and South. "north" and "south" are two edits
	// apart in a twenty-character string, so pure edit distance scored
	// "Atwima Nwabiagya South" against "Atwima Nwabiagya North" at 0.91 and
	// would have attached one district's boundary to the other — a silent,
	// invisible error affecting every containment query for both.
	//
	// If either name carries a directional or ordinal word, both must carry
	// the same ones.
	if !directionsAgree(a, b) {
		return 0
	}

	direct := similarity(a, b)

	// Same tokens in a different order is the same name.
	sorted := similarity(sortTokens(a), sortTokens(b))

	if sorted > direct {
		return sorted
	}
	return direct
}

// discriminatingTokens are words that, in Ghanaian administrative names, are
// the whole difference between two distinct places rather than a variant
// spelling of one.
var discriminatingTokens = map[string]bool{
	"north": true, "south": true, "east": true, "west": true,
	"central": true, "upper": true, "lower": true,
	"old": true, "new": true,
}

func directionsAgree(a, b string) bool {
	da, db := directionSet(a), directionSet(b)
	if len(da) != len(db) {
		return false
	}
	for k := range da {
		if !db[k] {
			return false
		}
	}
	return true
}

func directionSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, t := range strings.Fields(s) {
		if discriminatingTokens[t] {
			out[t] = true
		}
	}
	return out
}

func sortTokens(s string) string {
	t := strings.Fields(s)
	sort.Strings(t)
	return strings.Join(t, " ")
}
