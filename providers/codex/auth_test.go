package codex

import (
	"testing"
)

func TestExternalAccountIDIsAttributionOnly(t *testing.T) {
	raw := []byte(`{"tokens":{"account_id":"synthetic","access_token":"never-return-this"}}`)
	id, err := ExternalAccountID(raw)
	if err != nil || id != "synthetic" {
		t.Fatalf("unexpected account attribution: %q %v", id, err)
	}
	if _, err := ExternalAccountID([]byte(`{"tokens":{"access_token":"secret"}}`)); err == nil {
		t.Fatal("missing identity accepted")
	}
}
