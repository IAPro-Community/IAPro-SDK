package capacitymonitor

import (
	"errors"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

// ObservationKey keeps prior threshold state isolated by owner, full account
// identity, group and window; profile aliases alone are insufficient.
type ObservationKey struct {
	OwnerID string
	Scope   model.AccountScope
	Group   string
	Kind    string
}

type Observation struct {
	Key       ObservationKey
	State     State
	Reason    string
	Remaining *float64
	Observed  time.Time
	Expires   time.Time
}

// Observe is a pure read-model projection over a host-acquired snapshot. The
// host owns scheduling, authorization, persistence and alert delivery.
func Observe(owner string, snap model.UsageSnapshot, previous map[ObservationKey]State, thresholds Thresholds, now time.Time, maxAge time.Duration) ([]Observation, error) {
	scope := snap.AccountScope
	if owner == "" || !scope.Verifiable() || snap.ProviderID != scope.ProviderID || snap.ProfileID != scope.ProfileID ||
		now.IsZero() || maxAge <= 0 || thresholds.Validate() != nil {
		return nil, errors.New("invalid capacity observation context")
	}
	expiry := snap.FetchedAt.Add(maxAge)
	if snap.ExpiresAt != nil && snap.ExpiresAt.Before(expiry) {
		expiry = *snap.ExpiresAt
	}
	windows := snap.Windows
	if len(windows) == 0 {
		windows = []model.UsageWindow{{Kind: "unknown"}}
	}
	out := make([]Observation, 0, len(windows))
	seen := make(map[ObservationKey]bool)
	for _, window := range windows {
		key := ObservationKey{OwnerID: owner, Scope: scope, Group: window.Group, Kind: window.Kind}
		if window.Kind == "" || seen[key] {
			return nil, errors.New("missing or duplicate quota window")
		}
		seen[key] = true
		until := expiry
		if window.ResetTime != nil && window.ResetTime.Before(until) {
			until = *window.ResetTime
		}
		var remaining *float64
		if (snap.Status == model.UsageLive || snap.Status == model.UsageCached || snap.Status == model.UsageRateLimited) &&
			window.RemainingPercent != nil && *window.RemainingPercent >= 0 && *window.RemainingPercent <= 100 {
			v := *window.RemainingPercent
			remaining = &v
		}
		state, reason := EvaluateTransition(previous[key], remaining, snap.Status == model.UsageRateLimited, snap.FetchedAt, until, now, thresholds)
		if state == StateUnknown {
			remaining = nil
		}
		out = append(out, Observation{Key: key, State: state, Reason: reason, Remaining: remaining, Observed: snap.FetchedAt, Expires: until})
	}
	return out, nil
}
