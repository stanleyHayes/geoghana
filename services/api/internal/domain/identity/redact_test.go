package identity

import (
	"encoding/json"
	"strings"
	"testing"
)

// A key with no scopes is a normal thing to create: the portal's scope fieldset
// is plain checkboxes with no minimum-selection rule. It must not marshal to
// null, because the console calls .join on these and one null blanks the whole
// key-management screen — the screen you would use to revoke the offending key.
func TestRedactKeyEmptyCollectionsMarshalAsArrays(t *testing.T) {
	out := RedactKey(APIKey{ID: "key_1", Name: "no scopes"})

	if out.Scopes == nil {
		t.Fatal("Scopes is nil; it will marshal to null")
	}
	if out.AllowedOrigins == nil {
		t.Fatal("AllowedOrigins is nil; it will marshal to null")
	}
	if out.AllowedIPs == nil {
		t.Fatal("AllowedIPs is nil; it will marshal to null")
	}

	encoded, err := json.Marshal(map[string]any{
		"scopes":         out.Scopes,
		"allowedOrigins": out.AllowedOrigins,
		"allowedIps":     out.AllowedIPs,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "null") {
		t.Fatalf("empty collections still marshal to null: %s", encoded)
	}
}

// Populated collections must survive the copy unchanged.
func TestRedactKeyKeepsScopes(t *testing.T) {
	out := RedactKey(APIKey{ID: "key_2", Scopes: []Scope{"locations:read", "geocode:read"}})
	if len(out.Scopes) != 2 || out.Scopes[0] != "locations:read" {
		t.Fatalf("scopes not preserved: %v", out.Scopes)
	}
}
