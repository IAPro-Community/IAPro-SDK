package claude

import "testing"

func TestCredentialHintsNestedWithIdentityOnlyFlat(t *testing.T) {
	raw := []byte(`{"oauthAccount":{"email":"x@example.test"},"claudeAiOauth":{"accessToken":"secret","refreshToken":"secret2","expiresAt":1800000000000}}`)
	h, ok := ParseCredentialHints(raw)
	if !ok || !h.HasAccess || !h.HasRefresh || !h.HasExpiry || h.Email != "x@example.test" {
		t.Fatalf("unexpected hints: %+v", h)
	}
	h, ok = ParseCredentialHints([]byte(`{"email":"x@example.test"}`))
	if !ok || h.HasAccess || h.HasRefresh {
		t.Fatal("metadata alone is not credential evidence")
	}
}
