package account

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func codeAt(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	c, err := totp.GenerateCodeCustom(secret, at, totp.ValidateOpts{
		Period: totpPeriod, Skew: totpSkew, Digits: totpDigits, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestTOTPEnrolmentAndVerification(t *testing.T) {
	e, err := EnrolTOTP("GhanaGeo", "steward@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if e.Secret == "" || !strings.HasPrefix(e.URI, "otpauth://totp/") {
		t.Fatalf("bad enrolment: %+v", e)
	}
	if !strings.Contains(e.URI, "GhanaGeo") {
		t.Error("the issuer is missing from the otpauth URI — the app would show an unlabelled entry")
	}

	at := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	code := codeAt(t, e.Secret, at)

	step, err := VerifyTOTP(e.Secret, code, at)
	if err != nil {
		t.Fatalf("a freshly generated code was rejected: %v", err)
	}
	if step == 0 {
		t.Error("no time step was returned — replay could not be prevented")
	}

	t.Run("wrong code", func(t *testing.T) {
		if _, err := VerifyTOTP(e.Secret, "000000", at); !errors.Is(err, ErrTOTPInvalid) {
			t.Errorf("an incorrect code was accepted: %v", err)
		}
	})

	t.Run("no secret enrolled", func(t *testing.T) {
		if _, err := VerifyTOTP("", code, at); !errors.Is(err, ErrTOTPNotSetUp) {
			t.Errorf("verified against an empty secret: %v", err)
		}
	})

	t.Run("tolerates modest clock skew", func(t *testing.T) {
		if _, err := VerifyTOTP(e.Secret, code, at.Add(20*time.Second)); err != nil {
			t.Errorf("a code failed within the allowed skew: %v", err)
		}
	})

	t.Run("rejects a code from long ago", func(t *testing.T) {
		if _, err := VerifyTOTP(e.Secret, code, at.Add(10*time.Minute)); !errors.Is(err, ErrTOTPInvalid) {
			t.Errorf("a stale code was accepted: %v", err)
		}
	})
}

// Verification alone is not enough: a code stays valid for its whole window,
// so an observed code must not be usable a second time inside it.
func TestTOTPReplayIsRejected(t *testing.T) {
	e, _ := EnrolTOTP("GhanaGeo", "steward@example.com")
	at := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	code := codeAt(t, e.Secret, at)

	step, err := VerifyTOTP(e.Secret, code, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckTOTPReplay(step, 0); err != nil {
		t.Fatalf("first use rejected: %v", err)
	}
	// Same step again — the replay.
	if err := CheckTOTPReplay(step, step); !errors.Is(err, ErrTOTPReplayed) {
		t.Errorf("a replayed code was accepted: %v", err)
	}
	// An older step must not be accepted either; a clock rolled back a little
	// would otherwise reopen a used window.
	if err := CheckTOTPReplay(step-1, step); !errors.Is(err, ErrTOTPReplayed) {
		t.Errorf("an older step was accepted after a newer one: %v", err)
	}
	// A later step is fine.
	if err := CheckTOTPReplay(step+1, step); err != nil {
		t.Errorf("the next window was rejected: %v", err)
	}
}

func TestRecoveryCodes(t *testing.T) {
	plain, hashes, err := GenerateRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != RecoveryCodeCount || len(hashes) != RecoveryCodeCount {
		t.Fatalf("got %d codes and %d hashes", len(plain), len(hashes))
	}

	t.Run("stored hashed, never in plaintext", func(t *testing.T) {
		for i, h := range hashes {
			if h == plain[i] || strings.Contains(h, plain[i]) {
				t.Fatal("a recovery code was stored in plaintext")
			}
		}
	})

	t.Run("all distinct", func(t *testing.T) {
		seen := map[string]bool{}
		for _, c := range plain {
			if seen[c] {
				t.Fatal("duplicate recovery code generated")
			}
			seen[c] = true
		}
	})

	t.Run("single use", func(t *testing.T) {
		remaining, err := RedeemRecoveryCode(plain[3], hashes)
		if err != nil {
			t.Fatalf("a valid recovery code was rejected: %v", err)
		}
		if len(remaining) != len(hashes)-1 {
			t.Errorf("remaining = %d, want %d", len(remaining), len(hashes)-1)
		}
		// The same code must not work twice — this is the MFA-recovery flow
		// Spec §22.3 requires to be tested.
		if _, err := RedeemRecoveryCode(plain[3], remaining); !errors.Is(err, ErrTOTPInvalid) {
			t.Errorf("a recovery code was reused: %v", err)
		}
		// Other codes still work.
		if _, err := RedeemRecoveryCode(plain[0], remaining); err != nil {
			t.Errorf("an unrelated recovery code stopped working: %v", err)
		}
	})

	t.Run("unknown code rejected", func(t *testing.T) {
		if _, err := RedeemRecoveryCode("NOTAREALCODE", hashes); !errors.Is(err, ErrTOTPInvalid) {
			t.Errorf("an unknown recovery code was accepted: %v", err)
		}
	})
}
