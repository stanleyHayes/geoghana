package account

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

var (
	ErrTOTPInvalid  = errors.New("authentication code is incorrect")
	ErrTOTPReplayed = errors.New("that authentication code has already been used")
	ErrTOTPNotSetUp = errors.New("no authenticator is enrolled")
)

// TOTP parameters. 30-second period and 6 digits are what every authenticator
// app defaults to; changing them buys nothing and breaks scanning a QR code.
const (
	totpPeriod = 30
	totpDigits = otp.DigitsSix
	// totpSkew allows one step either side, tolerating a clock a little out
	// without widening the window enough to matter for brute force.
	totpSkew = 1
)

// TOTPEnrolment carries what the user needs to set up an authenticator.
type TOTPEnrolment struct {
	// Secret is stored on the account (encrypted at rest by the adapter).
	Secret string
	// URI is the otpauth:// value a QR code encodes.
	URI string
}

// EnrolTOTP generates a new authenticator secret for an account.
func EnrolTOTP(issuer, email string) (TOTPEnrolment, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: email,
		Period:      totpPeriod,
		Digits:      totpDigits,
		Algorithm:   otp.AlgorithmSHA1, // what authenticator apps actually support
	})
	if err != nil {
		return TOTPEnrolment{}, fmt.Errorf("generate TOTP secret: %w", err)
	}
	return TOTPEnrolment{Secret: key.Secret(), URI: key.URL()}, nil
}

// VerifyTOTP checks a code and returns the time step it was valid for.
//
// The step is returned so the caller can persist it and REJECT A REPLAY. A
// code stays valid for its whole 30-second window, so without recording the
// step, someone who observes a code — over a shoulder, in a log, on a phishing
// page — can use it again within that window. Verification alone is not enough.
func VerifyTOTP(secret, code string, at time.Time) (step uint64, err error) {
	if secret == "" {
		return 0, ErrTOTPNotSetUp
	}
	ok, err := totp.ValidateCustom(code, secret, at, totp.ValidateOpts{
		Period: totpPeriod, Skew: totpSkew, Digits: totpDigits, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil || !ok {
		return 0, ErrTOTPInvalid
	}
	// Which step matched: check the current one first, then the neighbours the
	// skew allows, so the recorded step is the one actually used.
	base := uint64(at.Unix()) / totpPeriod
	for _, candidate := range []uint64{base, base - 1, base + 1} {
		t := time.Unix(int64(candidate*totpPeriod), 0)
		if valid, _ := totp.ValidateCustom(code, secret, t, totp.ValidateOpts{
			Period: totpPeriod, Skew: 0, Digits: totpDigits, Algorithm: otp.AlgorithmSHA1,
		}); valid {
			return candidate, nil
		}
	}
	return base, nil
}

// CheckTOTPReplay rejects a code whose time step has already been consumed.
//
// lastStep is the highest step this account has successfully used. Requiring
// a strictly greater step also prevents a small clock rollback from replaying
// an older code.
func CheckTOTPReplay(step, lastStep uint64) error {
	if lastStep != 0 && step <= lastStep {
		return ErrTOTPReplayed
	}
	return nil
}

// RecoveryCodeCount is how many single-use codes are issued when MFA is
// enrolled. Ten is enough to survive losing a phone without becoming a
// long-lived list of passwords.
const RecoveryCodeCount = 10

// GenerateRecoveryCodes returns plaintext codes and their storage hashes.
//
// The plaintext is shown once. Codes are hashed like any other credential,
// because a leaked database must not hand over MFA bypass.
func GenerateRecoveryCodes() (plain []string, hashes []string, err error) {
	for i := 0; i < RecoveryCodeCount; i++ {
		b := make([]byte, 10)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, fmt.Errorf("generate recovery code: %w", err)
		}
		// Base32 without padding: unambiguous to read off a printout and
		// type back in, unlike base64 with its case-sensitive + and /.
		code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
		plain = append(plain, code)
		hashes = append(hashes, HashToken(code))
	}
	return plain, hashes, nil
}

// RedeemRecoveryCode finds and consumes a matching code, returning the
// remaining hashes. Comparison is constant time via TokenMatches.
func RedeemRecoveryCode(code string, hashes []string) ([]string, error) {
	for i, h := range hashes {
		if TokenMatches(code, h) {
			// Single use: the matched hash is removed, not marked.
			out := make([]string, 0, len(hashes)-1)
			out = append(out, hashes[:i]...)
			out = append(out, hashes[i+1:]...)
			return out, nil
		}
	}
	return hashes, ErrTOTPInvalid
}
