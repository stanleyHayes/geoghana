package ingest

import (
	"strings"
	"testing"
	"time"
)

func TestRunIdentityIsIdempotentForPayload(t *testing.T) {
	now := time.Date(2026, 8, 30, 1, 2, 3, 0, time.UTC)
	a, err := NewRun("geonames", strings.Repeat("a", 64), "req-1", now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewRun("geonames", strings.Repeat("a", 64), "a-different-transport-request", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID {
		t.Fatalf("same import got different ids: %s != %s", a.ID, b.ID)
	}
}

func TestRawRecordStripsFreeFormFailureDetails(t *testing.T) {
	run, _ := NewRun("provider", strings.Repeat("a", 64), "req", time.Now())
	record, err := NewRawRecord(run, "provider:42", strings.Repeat("b", 64), RecordRejected,
		"unresolved region: person@example.com", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if record.ReasonCode != "unresolved_region" {
		t.Fatalf("reason = %q", record.ReasonCode)
	}
}

func TestRawRecordRequiresDigest(t *testing.T) {
	run, _ := NewRun("provider", "", "req", time.Now())
	if _, err := NewRawRecord(run, "42", "payload", RecordCreated, "", time.Now()); err == nil {
		t.Fatal("accepted unhashed raw record")
	}
}
