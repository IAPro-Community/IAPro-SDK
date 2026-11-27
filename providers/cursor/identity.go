// Package cursor parses local provider metadata. These are identity hints,
// not proof of authentication or verified JWT signatures.
package cursor

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

const maxMetadataBytes = 4 * 1024 * 1024

type IdentityHints struct {
	Email      string
	ExternalID string
}

// IdentityFromMetadata returns only account attribution. The host owns
// authorized reads, wiping the supplied buffers and verifying active login.
func IdentityFromMetadata(config, auth []byte) IdentityHints {
	var out IdentityHints
	if len(config) > 0 && len(config) <= maxMetadataBytes {
		var c struct {
			AuthInfo struct {
				Email  string `json:"email"`
				AuthID string `json:"authId"`
			} `json:"authInfo"`
		}
		if json.Unmarshal(config, &c) == nil {
			out.Email = strings.TrimSpace(c.AuthInfo.Email)
			out.ExternalID = strings.TrimSpace(c.AuthInfo.AuthID)
		}
	}
	if out.ExternalID != "" || len(auth) == 0 || len(auth) > maxMetadataBytes {
		return out
	}
	var a struct {
		AccessToken string `json:"accessToken"`
	}
	if json.Unmarshal(auth, &a) != nil {
		return out
	}
	parts := strings.Split(a.AccessToken, ".")
	if len(parts) != 3 || len(parts[1]) > maxMetadataBytes {
		return out
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	defer clear(raw)
	if err != nil {
		return out
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if json.Unmarshal(raw, &claims) == nil {
		out.ExternalID = strings.TrimSpace(claims.Sub)
	}
	return out
}
