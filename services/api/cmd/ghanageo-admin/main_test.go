package main

import "testing"

func TestSeedCountsAllowLicensedPlaceEnrichment(t *testing.T) {
	if problems := seedCountProblems(16, 261, 15941); len(problems) != 0 {
		t.Fatalf("enriched corpus failed bootstrap counts: %v", problems)
	}
	if problems := seedCountProblems(16, 261, 15); len(problems) != 1 {
		t.Fatalf("missing bootstrap place was not detected: %v", problems)
	}
	if problems := seedCountProblems(15, 260, 16); len(problems) != 2 {
		t.Fatalf("administrative count drift was not detected: %v", problems)
	}
}
