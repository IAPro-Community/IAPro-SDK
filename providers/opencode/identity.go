package opencode

import (
	"encoding/json"
	"strings"
)

// HasCredentialMetadata checks only the local document structure; it is not
// a remote authentication check.
func HasCredentialMetadata(raw []byte) bool {
	if len(raw) == 0 || len(raw) > 4*1024*1024 {
		return false
	}
	var entries map[string]struct {
		Key    string `json:"key"`
		Token  string `json:"token"`
		Access string `json:"access"`
	}
	if json.Unmarshal(raw, &entries) != nil {
		return false
	}
	for _, entry := range entries {
		if strings.TrimSpace(entry.Key) != "" || strings.TrimSpace(entry.Token) != "" || strings.TrimSpace(entry.Access) != "" {
			return true
		}
	}
	return false
}
