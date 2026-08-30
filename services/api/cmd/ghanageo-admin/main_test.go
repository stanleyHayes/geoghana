package main

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestSHA256FileHashesTheCompletePayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte("abc\nsecond row\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "3f95ab38feae47eaf88936d51c5f9eb3e08a907f3b007871c014a3befb36d5a1"; got != want {
		t.Fatalf("digest = %s, want %s", got, want)
	}
}

func TestCLIRequestIDsAreOpaqueAndUnique(t *testing.T) {
	a, err := newCLIRequestID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := newCLIRequestID()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 36 || a[:4] != "cli_" || a == b {
		t.Fatalf("invalid request ids %q and %q", a, b)
	}
}
