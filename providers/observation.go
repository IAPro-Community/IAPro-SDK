package providers

import (
	"strings"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

// ObservationFresh is a time-bound read-model check, not dispatch evidence.
func ObservationFresh(s model.UsageSnapshot, now time.Time, ttl time.Duration) bool {
	return ttl > 0 && !now.IsZero() && !s.FetchedAt.IsZero() && !s.FetchedAt.After(now) &&
		now.Before(s.FetchedAt.Add(ttl)) && (s.ExpiresAt == nil || now.Before(*s.ExpiresAt))
}

// ReusableIdleObservation avoids repeating an idle monitor probe. The host
// must check identity and active runtime separately before reuse.
func ReusableIdleObservation(s model.UsageSnapshot, now time.Time, ttl time.Duration) bool {
	if !ObservationFresh(s, now, ttl) {
		return false
	}
	switch s.Status {
	case model.UsageLive, model.UsageCached:
		return len(s.Windows) > 0
	case model.UsageRateLimited:
		return true
	default:
		return false
	}
}

// DuplicateIdentityHint preserves historical display/deduplication hints for
// account management. It is NOT verified identity, authorization, or evidence
// for deleting profiles, transferring credentials or switching accounts.
func DuplicateIdentityHint(provider, email, externalID string, authenticated bool) string {
	if !authenticated {
		return ""
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	email = strings.ToLower(strings.TrimSpace(email))
	externalID = strings.ToLower(strings.TrimSpace(externalID))
	if (provider == "codex" || provider == "cursor") && externalID != "" {
		return externalID
	}
	if email != "" {
		return email
	}
	return externalID
}
