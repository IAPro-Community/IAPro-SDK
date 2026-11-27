package zcode

import (
	"encoding/json"
	"strings"
)

// HasCredentialMetadata is a structural hint, not proof of an active login.
func HasCredentialMetadata(raw []byte) bool {
	if len(raw) == 0 || len(raw) > 4*1024*1024 {
		return false
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil {
		return false
	}
	// A provider selector alone is not a credential.
	for _, key := range []string{"oauth:zai:access_token", "zcodejwttoken"} {
		var token string
		if json.Unmarshal(doc[key], &token) == nil && strings.TrimSpace(token) != "" {
			return true
		}
	}
	return false
}
