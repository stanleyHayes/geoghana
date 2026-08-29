package geography

import (
	"errors"
	"testing"
)

func TestStableIDIsDeterministicULIDAndNamespaced(t *testing.T) {
	first, err := StableID("region", "seed-bootstrap", "gh-region-ahafo")
	if err != nil {
		t.Fatal(err)
	}
	second, _ := StableID("region", "seed-bootstrap", "gh-region-ahafo")
	if first != second || !IsULID(first) {
		t.Fatalf("stable id = %q / %q", first, second)
	}
	otherEntity, _ := StableID("place", "seed-bootstrap", "gh-region-ahafo")
	otherSource, _ := StableID("region", "another-source", "gh-region-ahafo")
	if first == otherEntity || first == otherSource {
		t.Fatal("entity/source namespaces collided")
	}
}

func TestStableIDRejectsIncompleteSourceIdentity(t *testing.T) {
	for _, input := range [][3]string{{"", "source", "id"}, {"place", "", "id"}, {"place", "source", ""}} {
		if _, err := StableID(input[0], input[1], input[2]); !errors.Is(err, ErrInvalidStableIDInput) {
			t.Fatalf("input %q returned %v", input, err)
		}
	}
}
