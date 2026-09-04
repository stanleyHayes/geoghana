package account

import (
	"testing"
	"time"
)

func TestPasswordLocked(t *testing.T) {
	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	future := now.Add(5 * time.Minute)
	past := now.Add(-5 * time.Minute)

	for _, tc := range []struct {
		name string
		acc  Account
		want bool
	}{
		{"never locked", Account{}, false},
		{"lock in the future", Account{LockedUntil: &future}, true},
		{"lock already expired", Account{LockedUntil: &past}, false},
		{"lock expiring exactly now", Account{LockedUntil: &now}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.acc.PasswordLocked(now); got != tc.want {
				t.Fatalf("PasswordLocked = %v, want %v", got, tc.want)
			}
		})
	}
}

// The lockout must be long enough to matter and the threshold low enough to bite
// before a guessing run gets anywhere, without locking out a person who simply
// mistyped. These are the numbers the login flow depends on.
func TestLockoutConstantsAreSane(t *testing.T) {
	if MaxFailedLogins < 3 || MaxFailedLogins > 12 {
		t.Fatalf("MaxFailedLogins = %d, outside a sane range", MaxFailedLogins)
	}
	if LoginLockout < time.Minute {
		t.Fatalf("LoginLockout = %s, too short to slow anything down", LoginLockout)
	}
}
