package cursor

import (
	"encoding/base64"
	"testing"
)

func TestIdentityHints(t *testing.T) {
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"from-jwt"}`))
	auth := []byte(`{"accessToken":"hdr.` + claims + `.sig"}`)
	cfg := []byte(`{"authInfo":{"email":"account@example.test","authId":"from-config"}}`)
	if got := IdentityFromMetadata(cfg, auth); got.ExternalID != "from-config" || got.Email != "account@example.test" {
		t.Fatalf("wrong config hints: %+v", got)
	}
	if got := IdentityFromMetadata(nil, auth); got.ExternalID != "from-jwt" {
		t.Fatalf("wrong JWT hint: %+v", got)
	}
	if got := IdentityFromMetadata(nil, []byte(`{"accessToken":"invalid"}`)); got.ExternalID != "" {
		t.Fatal("invalid token must not provide identity")
	}
}
