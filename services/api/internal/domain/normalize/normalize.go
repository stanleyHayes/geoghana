// Package normalize turns Ghanaian place names into deterministic search tokens.
//
// The same input must ALWAYS produce the same output (Spec 22.1). Folding is a
// search-index concern only: display always preserves the true orthography,
// including the Twi/Ga/Ewe characters this package folds away.
package normalize

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// ghanaianFolds maps the Twi/Ga/Ewe characters that carry meaning in display
// but must collapse for matching. Unicode NFD does not decompose these — they
// are distinct letters, not accented Latin — so they are folded explicitly.
var ghanaianFolds = map[rune]string{
	'ɔ': "o", 'Ɔ': "o",
	'ɛ': "e", 'Ɛ': "e",
	'ŋ': "n", 'Ŋ': "n",
	'ɖ': "d", 'Ɖ': "d",
	'ƒ': "f", 'Ƒ': "f",
	'ʋ': "v", 'Ʋ': "v",
	'ɣ': "g", 'Ɣ': "g",
}

// abbreviations expands common Ghanaian shorthand so "tema comm 25" reaches
// "Tema Community 25" (Spec 10).
var abbreviations = map[string]string{
	"comm": "community", "comms": "community",
	"rd": "road", "st": "street", "ave": "avenue", "jn": "junction", "jct": "junction",
	"ext": "extension", "est": "estate", "ests": "estate",
	"n": "north", "s": "south", "e": "east", "w": "west",
	"gt": "greater", "accra": "accra",
	"no": "number", "nr": "near",
}

// Name normalizes a place name to a deterministic matching token.
func Name(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if folded, ok := ghanaianFolds[r]; ok {
			b.WriteString(folded)
			continue
		}
		b.WriteRune(r)
	}
	s = b.String()

	// Strip combining marks after NFD so "Kwabɛnyá" and "Kwabenya" agree.
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	if out, _, err := transform.String(t, s); err == nil {
		s = out
	}
	s = strings.ToLower(s)

	// Collapse anything that is not a letter or digit into a single space.
	var parts []string
	var cur strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else if cur.Len() > 0 {
			parts = append(parts, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}

	for i, p := range parts {
		if exp, ok := abbreviations[p]; ok {
			parts[i] = exp
		}
	}
	return strings.Join(parts, " ")
}

// Tokens returns the normalized tokens of a name.
func Tokens(s string) []string {
	n := Name(s)
	if n == "" {
		return nil
	}
	return strings.Split(n, " ")
}
