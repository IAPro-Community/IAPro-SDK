// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus app-server usage mapping; see NOTICE.

package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers/spec"
)

type rateWindow struct {
	UsedPercent        *float64 `json:"usedPercent"`
	WindowDurationMins *int     `json:"windowDurationMins"`
	ResetsAt           *int64   `json:"resetsAt"`
}

type rateBucket struct {
	Primary              *rateWindow `json:"primary"`
	Secondary            *rateWindow `json:"secondary"`
	NormalModelSlug      *string     `json:"normalModelSlug"`
	PlanType             *string     `json:"planType"`
	SpendControlReached  *bool       `json:"spendControlReached"`
	RateLimitReachedType *string     `json:"rateLimitReachedType"`
}

type rateResponse struct {
	OrdinaryUsageAllowed *bool                 `json:"ordinaryUsageAllowed"`
	RateLimits           rateBucket            `json:"rateLimits"`
	RateLimitsByLimitID  map[string]rateBucket `json:"rateLimitsByLimitId"`
	AccountID            *string               `json:"accountId"`
}

// ParseUsage maps the result of account/rateLimits/read. The trusted host must
// first verify app-server CODEX_HOME and derive scope from authenticated identity.
// expectedExternalID is the independently verified provider account ID, not a
// profile label. No network call or credential read occurs in this package.
func ParseUsage(raw []byte, scope model.AccountScope, expectedExternalID string, observedAt time.Time) (model.UsageSnapshot, error) {
	if scope.ProviderID != string(model.ProviderCodex) || !scope.Verifiable() || strings.TrimSpace(expectedExternalID) == "" || observedAt.IsZero() {
		return model.UsageSnapshot{}, errors.New("codex quota identity or observation time missing")
	}
	if len(raw) > 4*1024*1024 {
		return model.UsageSnapshot{}, errors.New("codex quota response too large")
	}
	var resp rateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return model.UsageSnapshot{}, errors.New("invalid codex quota JSON")
	}
	if resp.AccountID == nil || strings.TrimSpace(*resp.AccountID) != strings.TrimSpace(expectedExternalID) {
		return model.UsageSnapshot{}, errors.New("codex quota account identity mismatch")
	}
	bucket := resp.RateLimits
	if b, ok := resp.RateLimitsByLimitID["codex"]; ok {
		bucket = b
	} else if len(resp.RateLimitsByLimitID) != 0 {
		// A different named pool must never be attributed to Codex.
		return model.UsageSnapshot{}, errors.New("codex quota bucket missing")
	}
	snap := model.UsageSnapshot{
		ProviderID: scope.ProviderID, ProfileID: scope.ProfileID, AccountScope: scope,
		Source: model.SourceOfficialAPI, Status: model.UsageLive,
		Confidence: model.UsageConfidenceHigh, FetchedAt: observedAt.UTC(),
	}
	if bucket.NormalModelSlug != nil {
		snap.ModelName = *bucket.NormalModelSlug
	}
	if bucket.PlanType != nil {
		snap.Plan = *bucket.PlanType
	}
	// Derived from the provider spec instead of a second hard-coded table:
	// the group key names the quota pool the windows belong to.
	group, _ := spec.MustGet("codex").DefaultModelGroup()
	for _, item := range []struct {
		window *rateWindow
		kind   string
	}{{bucket.Primary, "5h"}, {bucket.Secondary, "weekly"}} {
		w := item.window
		if w == nil {
			continue
		}
		kind := item.kind
		if w.WindowDurationMins != nil {
			switch *w.WindowDurationMins {
			case 300:
				kind = "5h"
			case 10080:
				kind = "weekly"
			default:
				kind = fmt.Sprintf("minutes_%d", *w.WindowDurationMins)
			}
		}
		window := model.UsageWindow{Kind: kind, Group: group}
		if w.UsedPercent != nil && *w.UsedPercent >= 0 && *w.UsedPercent <= 100 {
			used := *w.UsedPercent
			remaining := 100 - used
			window.UsedPercent = &used
			window.RemainingPercent = &remaining
		}
		if w.ResetsAt != nil && *w.ResetsAt > 0 {
			reset := time.Unix(*w.ResetsAt, 0).UTC()
			window.ResetTime = &reset
			window.ResetDescription = reset.Format(time.RFC3339)
		}
		snap.Windows = append(snap.Windows, window)
	}
	switch {
	case resp.OrdinaryUsageAllowed != nil && !*resp.OrdinaryUsageAllowed,
		bucket.SpendControlReached != nil && *bucket.SpendControlReached,
		bucket.RateLimitReachedType != nil && strings.TrimSpace(*bucket.RateLimitReachedType) != "":
		snap.Status = model.UsageRateLimited
		snap.Error = "provider blocked ordinary usage or spend"
	case len(snap.Windows) == 0:
		snap.Status = model.UsageUnknown
		snap.Confidence = model.UsageConfidenceUnknown
	}
	return snap, nil
}
