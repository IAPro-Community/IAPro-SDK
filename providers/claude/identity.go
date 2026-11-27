package claude

import (
	"encoding/json"
	"strings"
	"time"
)

// CredentialHints contains only local metadata, never token values. It cannot
// establish remote authentication, authorize dispatch or prove account identity.
type CredentialHints struct {
	Email      string
	HasAccess  bool
	HasRefresh bool
	ExpiresAt  time.Time
	HasExpiry  bool
}

type credentialFields struct {
	Email        string `json:"email"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    any    `json:"expiresAt"`
	Expiry       any    `json:"expiry"`
}

func ParseCredentialHints(raw []byte) (CredentialHints, bool) {
	if len(raw) == 0 || len(raw) > 4*1024*1024 {
		return CredentialHints{}, false
	}
	var doc struct {
		credentialFields
		OAuthAccount struct {
			Email string `json:"email"`
		} `json:"oauthAccount"`
		Nested credentialFields `json:"claudeAiOauth"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return CredentialHints{}, false
	}
	fields := doc.credentialFields
	if strings.TrimSpace(fields.AccessToken) == "" && strings.TrimSpace(fields.RefreshToken) == "" &&
		(strings.TrimSpace(doc.Nested.AccessToken) != "" || strings.TrimSpace(doc.Nested.RefreshToken) != "") {
		fields = doc.Nested
	}
	h := CredentialHints{Email: strings.TrimSpace(fields.Email), HasAccess: strings.TrimSpace(fields.AccessToken) != "", HasRefresh: strings.TrimSpace(fields.RefreshToken) != ""}
	if email := strings.TrimSpace(doc.OAuthAccount.Email); email != "" && h.Email == "" {
		h.Email = email
	}
	h.ExpiresAt, h.HasExpiry = ParseCredentialExpiry(fields.ExpiresAt)
	if !h.HasExpiry {
		h.ExpiresAt, h.HasExpiry = ParseCredentialExpiry(fields.Expiry)
	}
	return h, h.Email != "" || h.HasAccess || h.HasRefresh
}

func ParseCredentialExpiry(raw any) (time.Time, bool) {
	switch v := raw.(type) {
	case float64:
		if v > 1e12 && v < 253402300800000 {
			return time.UnixMilli(int64(v)).UTC(), true
		}
		if v > 1e9 && v < 253402300800 {
			return time.Unix(int64(v), 0).UTC(), true
		}
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return ParseCredentialExpiry(f)
		}
	case string:
		if t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(v)); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
