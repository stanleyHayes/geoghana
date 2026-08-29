package seed

import (
	"testing"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
)

// A re-seed must not undo a steward's work.
//
// Seeding is an upsert, so before this guard a second `data seed` rewrote
// every field of every record — reverting a correction and demoting the
// verification status back to the CSV's, with no audit row to show it
// happened. Rule R5 forbids promoting a seed row to canonical automatically;
// the reverse is worse, because it destroys something a human did on purpose.
func TestStewardOwnedRecordsAreProtected(t *testing.T) {
	cases := []struct {
		status    geography.VerificationStatus
		protected bool
	}{
		// Only a steward can set these two, so only these are protected.
		{geography.VerificationReviewed, true},
		{geography.VerificationCanonical, true},
		// These are what the seed itself writes; re-importing them is the
		// idempotency the seed is supposed to have.
		{geography.VerificationReference, false},
		{geography.VerificationNeedsRecon, false},
		// An unrecognised value must not be treated as steward-owned, or a
		// typo in a CSV would make a record permanently un-seedable.
		{geography.VerificationStatus("SOMETHING_ELSE"), false},
	}
	for _, c := range cases {
		if got := stewardOwned(c.status); got != c.protected {
			t.Errorf("stewardOwned(%s) = %v, want %v", c.status, got, c.protected)
		}
	}
}
