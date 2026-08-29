package geography

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

var ErrInvalidStableIDInput = errors.New("stable id inputs must not be empty")

// stableIDEpoch is fixed deliberately. Geography identifiers represent
// long-lived source identities, not the wall-clock instant an import happened;
// using import time would create a different id on every replay.
var stableIDEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// StableID returns a deterministic, standards-compliant ULID for one source
// record. Entity kind is part of the namespace so equal external ids in two
// collections cannot collide.
func StableID(entity, source, externalID string) (string, error) {
	entity = strings.ToLower(strings.TrimSpace(entity))
	source = strings.ToLower(strings.TrimSpace(source))
	externalID = strings.TrimSpace(externalID)
	if entity == "" || source == "" || externalID == "" {
		return "", ErrInvalidStableIDInput
	}
	sum := sha256.Sum256([]byte("ghanageo:v1:" + entity + ":" + source + ":" + externalID))
	id, err := ulid.New(ulid.Timestamp(stableIDEpoch), bytes.NewReader(sum[:10]))
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func IsULID(value string) bool {
	_, err := ulid.ParseStrict(strings.TrimSpace(value))
	return err == nil
}
