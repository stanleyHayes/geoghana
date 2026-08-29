package audit

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// An entry that cannot say who did what to which thing is not evidence, so
// New must refuse it rather than write a partial row.
func TestNewRejectsIncompleteEntries(t *testing.T) {
	good := Actor{Kind: ActorOperator, ID: "shayford"}
	target := Target{Kind: "dataset", ID: "2026.08.1-seed"}

	cases := []struct {
		name   string
		actor  Actor
		action Action
		target Target
		want   error
	}{
		{"no actor kind", Actor{ID: "x"}, ActionKeyCreated, target, ErrNoActor},
		{"no actor id", Actor{Kind: ActorOperator}, ActionKeyCreated, target, ErrNoActor},
		{"no action", good, "", target, ErrNoAction},
		{"blank action", good, "   ", target, ErrNoAction},
		{"no target kind", good, ActionKeyCreated, Target{ID: "x"}, ErrNoTarget},
		{"no target id", good, ActionKeyCreated, Target{Kind: "dataset"}, ErrNoTarget},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := New(c.actor, c.action, c.target); !errors.Is(err, c.want) {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}

	if _, err := New(good, ActionKeyCreated, target); err != nil {
		t.Errorf("a complete entry was rejected: %v", err)
	}
}

// A privileged action that FAILED is often the more interesting row — a
// rejected revocation is a security signal, and dropping it hides exactly
// what a review is looking for.
func TestFailedActionsAreRecordable(t *testing.T) {
	e, err := New(Actor{Kind: ActorAdmin, ID: "u1"}, ActionKeyRevoked, Target{Kind: "api_key", ID: "k1"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Outcome != OutcomeSucceeded {
		t.Errorf("default outcome = %q, want succeeded", e.Outcome)
	}
	f := e.Failed(errors.New("permission denied"))
	if f.Outcome != OutcomeFailed || f.Error != "permission denied" {
		t.Errorf("Failed() did not record the rejection: %+v", f)
	}
	// The original must be unchanged — builders return copies, so a caller
	// cannot accidentally mutate an entry another goroutine is writing.
	if e.Outcome != OutcomeSucceeded {
		t.Error("Failed() mutated the receiver")
	}
}

// Redaction happens on the way IN. An audit row is read by more people than
// the record it describes, so a credential in a before/after snapshot would
// be exposed far more widely than the secret itself ever was.
func TestRedactRemovesSecrets(t *testing.T) {
	in := map[string]any{
		"name":          "Production key",
		"secret":        "gh_live_abc123",
		"secretHash":    "$argon2id$...",
		"apiKey":        "gh_live_def456",
		"Authorization": "Bearer xyz",
		"password":      "hunter2",
		"scopes":        []string{"locations:read"},
		"nested": map[string]any{
			"token": "nested-secret",
			"safe":  "keep me",
		},
	}
	got := Redact(in)

	for _, k := range []string{"secret", "secretHash", "apiKey", "Authorization", "password"} {
		if got[k] != "[redacted]" {
			t.Errorf("%s = %v, want [redacted]", k, got[k])
		}
	}
	if got["name"] != "Production key" {
		t.Errorf("non-secret field was redacted: %v", got["name"])
	}
	if !reflect.DeepEqual(got["scopes"], []string{"locations:read"}) {
		t.Errorf("scopes altered: %v", got["scopes"])
	}
	// A secret one level down is still a secret.
	nested, ok := got["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested map lost: %T", got["nested"])
	}
	if nested["token"] != "[redacted]" {
		t.Errorf("nested token = %v, want [redacted]", nested["token"])
	}
	if nested["safe"] != "keep me" {
		t.Errorf("nested safe value altered: %v", nested["safe"])
	}

	// The input must not be modified in place; the caller may still need it.
	if in["secret"] != "gh_live_abc123" {
		t.Error("Redact mutated its argument")
	}
	if Redact(nil) != nil {
		t.Error("Redact(nil) should stay nil")
	}
}

// The whole point of the package: no exported method changes a stored entry's
// identity or history. Builders return copies and there is no setter for ID,
// At, Actor, Action or Target.
func TestNoMutatorsForIdentityFields(t *testing.T) {
	forbidden := map[string]bool{
		"SetID": true, "SetAt": true, "SetActor": true,
		"SetAction": true, "SetTarget": true, "Delete": true, "Update": true,
	}
	et := reflect.TypeOf(Entry{})
	for i := 0; i < et.NumMethod(); i++ {
		if forbidden[et.Method(i).Name] {
			t.Errorf("Entry exposes a mutator %q — the log must be append-only",
				et.Method(i).Name)
		}
	}
}

// helper: build a valid chain the way the repository does.
func chainOf(n int) []Entry {
	prev := GenesisHash
	out := make([]Entry, 0, n)
	for i := 0; i < n; i++ {
		e, _ := New(Actor{Kind: ActorOperator, ID: "op"}, ActionKeyCreated,
			Target{Kind: "api_key", ID: fmt.Sprintf("k%d", i)})
		e.ID = fmt.Sprintf("aud_%d", i)
		e.At = time.Unix(int64(1700000000+i), 0).UTC()
		e.PrevHash = prev
		e.Hash = e.ComputeHash()
		prev = e.Hash
		out = append(out, e)
	}
	return out
}

func TestVerifyChainAcceptsAnIntactLog(t *testing.T) {
	if err := VerifyChain(chainOf(5)); err != nil {
		t.Errorf("intact chain rejected: %v", err)
	}
	if err := VerifyChain(nil); err != nil {
		t.Errorf("empty chain rejected: %v", err)
	}
}

// The property the whole design rests on: an edit anywhere is detectable.
func TestVerifyChainDetectsTampering(t *testing.T) {
	t.Run("edited content", func(t *testing.T) {
		c := chainOf(5)
		// The classic cover-up: change what an action was, leave hashes alone.
		c[2].Action = ActionKeyRevoked
		err := VerifyChain(c)
		if err == nil {
			t.Fatal("an edited entry verified clean")
		}
		if !strings.Contains(err.Error(), "modified after it was written") {
			t.Errorf("unhelpful error: %v", err)
		}
	})

	t.Run("deleted row", func(t *testing.T) {
		c := chainOf(5)
		c = append(c[:2], c[3:]...) // remove entry 2
		if err := VerifyChain(c); err == nil {
			t.Fatal("a deleted entry went undetected")
		}
	})

	t.Run("inserted row", func(t *testing.T) {
		c := chainOf(5)
		fake, _ := New(Actor{Kind: ActorOperator, ID: "attacker"}, ActionKeyCreated,
			Target{Kind: "api_key", ID: "backdoor"})
		fake.ID = "aud_fake"
		fake.At = time.Unix(1700000002, 0).UTC()
		fake.PrevHash = c[1].Hash
		fake.Hash = fake.ComputeHash()
		c = append(c[:2], append([]Entry{fake}, c[2:]...)...)
		if err := VerifyChain(c); err == nil {
			t.Fatal("an inserted entry went undetected")
		}
	})

	t.Run("truncated head", func(t *testing.T) {
		// Removing the first rows is why genesis is a sentinel rather than "".
		c := chainOf(5)[2:]
		if err := VerifyChain(c); err == nil {
			t.Fatal("a truncated log verified clean")
		}
	})

	t.Run("rehashed after edit still breaks the chain", func(t *testing.T) {
		// A tamperer who recomputes the edited row's own hash still cannot fix
		// the rows after it without recomputing the entire tail.
		c := chainOf(5)
		c[1].Action = ActionKeyRevoked
		c[1].Hash = c[1].ComputeHash()
		if err := VerifyChain(c); err == nil {
			t.Fatal("a rehashed edit went undetected")
		}
	})
}

// Hashing must be deterministic, or every verification would fail at random.
// Go randomises map iteration, so Before/After need sorted-key rendering.
func TestCanonicalIsDeterministicAcrossMapOrder(t *testing.T) {
	mk := func() Entry {
		e, _ := New(Actor{Kind: ActorAdmin, ID: "u"}, ActionRecordUpdated,
			Target{Kind: "place", ID: "gh-place-osu"})
		e.ID, e.At = "aud_1", time.Unix(1700000000, 0).UTC()
		e.BeforeJSON, _ = CanonicalJSON(map[string]any{"z": 1, "a": 2, "m": map[string]any{"q": 1, "b": 2}})
		e.AfterJSON, _ = CanonicalJSON(map[string]any{"a": 9, "z": 8, "m": map[string]any{"b": 3, "q": 4}})
		return e
	}
	first := mk().ComputeHash()
	for i := 0; i < 50; i++ {
		if got := mk().ComputeHash(); got != first {
			t.Fatalf("hash unstable across runs: %s != %s", got, first)
		}
	}
}
