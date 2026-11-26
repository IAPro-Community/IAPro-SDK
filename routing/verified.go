// SPDX-License-Identifier: MIT

package routing

import (
	"errors"
	"strings"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers/spec"
	"github.com/IAPro-Community/IAPro-SDK/quota"
)

var ErrUnverifiedQuota = errors.New("quota or model identity is not verified")

// EvidenceInput contains facts independently established by the authorized
// host. It deliberately carries the original snapshot, not a reconstructed DTO.
type EvidenceInput struct {
	OwnerID       string
	Scope         model.AccountScope
	Snapshot      model.UsageSnapshot
	Model         string
	Now           time.Time
	Authorized    bool
	TermsPermit   bool
	Authenticated bool
	Paid          bool
	Health        string
}

// VerifiedAccount builds conservative single-pool dispatch evidence. It does
// not infer model compatibility from a family alias or a subscription name.
// A multi-pool provider needs an independently verified exact mapping adapter.
func VerifiedAccount(in EvidenceInput) (Account, error) {
	s := in.Snapshot
	if in.OwnerID == "" || !in.Scope.Verifiable() || in.Now.IsZero() || !in.Authenticated ||
		in.Scope != s.AccountScope || s.ProviderID != in.Scope.ProviderID || s.ProfileID != in.Scope.ProfileID ||
		s.Source != model.SourceOfficialAPI || s.Confidence != model.UsageConfidenceHigh ||
		strings.TrimSpace(in.Model) == "" || s.ModelName != in.Model ||
		s.FetchedAt.IsZero() || s.FetchedAt.After(in.Now) || s.ExpiresAt == nil ||
		!in.Now.Before(*s.ExpiresAt) || !s.ExpiresAt.After(s.FetchedAt) ||
		len(s.Windows) == 0 {
		return Account{}, ErrUnverifiedQuota
	}
	switch s.Status {
	case model.UsageLive, model.UsageCached, model.UsageRateLimited:
	default:
		return Account{}, ErrUnverifiedQuota
	}
	group := s.Windows[0].Group
	providerSpec, ok := spec.Get(in.Scope.ProviderID)
	if !ok || len(providerSpec.ModelGroups) != 1 || providerSpec.ModelGroups[0].Key != group {
		return Account{}, ErrUnverifiedQuota
	}
	kinds := make(map[string]bool, len(s.Windows))
	for _, w := range s.Windows {
		if w.Group != group || strings.TrimSpace(w.Kind) == "" || w.Kind == "unknown" || kinds[w.Kind] ||
			w.RemainingPercent == nil || !(*w.RemainingPercent >= 0 && *w.RemainingPercent <= 100) ||
			(w.ResetTime != nil && !in.Now.Before(*w.ResetTime)) {
			return Account{}, ErrUnverifiedQuota
		}
		kinds[w.Kind] = true
	}
	v := quota.BuildQuotaView(s, "", "")
	v.ComputeAvailabilityAt(in.Now)
	return Account{
		OwnerID: in.OwnerID, Scope: in.Scope, QuotaScope: s.AccountScope, Model: in.Model,
		QuotaGroup: group, GroupMapped: true, Authorized: in.Authorized, TermsPermit: in.TermsPermit,
		ModelCompatible: true, Paid: in.Paid, ExpiresAt: *s.ExpiresAt,
		Eligibility: quota.EligibilityInput{View: v, Authenticated: in.Authenticated, Health: in.Health, Now: in.Now},
	}, nil
}

// PrimaryState distinguishes a verified exhausted primary from unknown or
// stale evidence. Only Exhausted permits the host to consider a pre-dispatch
// fallback; SelectFallback must still authorize the actual destination.
type PrimaryState string

const (
	PrimaryAvailable PrimaryState = "AVAILABLE"
	PrimaryExhausted PrimaryState = "EXHAUSTED"
	PrimaryUnknown   PrimaryState = "UNKNOWN"
)

func AssessPrimary(a Account, now time.Time) PrimaryState {
	v := a.Eligibility.View
	if now.IsZero() || a.OwnerID == "" || !a.Scope.Verifiable() || !a.Authorized || !a.TermsPermit ||
		!a.ModelCompatible || !a.GroupMapped || strings.TrimSpace(a.Model) == "" ||
		a.Scope != a.QuotaScope || a.Scope != v.Scope || v.Provider != a.Scope.ProviderID ||
		v.Profile != a.Scope.ProfileID || v.FetchedAt.IsZero() || v.FetchedAt.After(now) ||
		v.ExpiresAt == nil || !now.Before(*v.ExpiresAt) || !now.Before(a.ExpiresAt) ||
		a.ExpiresAt.After(*v.ExpiresAt) || !a.Eligibility.Authenticated {
		return PrimaryUnknown
	}
	in := a.Eligibility
	in.Now, in.ExplicitSelection = now, false
	in.View.ComputeAvailabilityAt(now)
	if v.Status == string(model.UsageRateLimited) {
		return PrimaryExhausted
	}
	if v.Status != string(model.UsageLive) && v.Status != string(model.UsageCached) {
		return PrimaryUnknown
	}
	if modelPoolAvailable(in.View, a.QuotaGroup, now) && quota.EvaluateEligibility(in).Eligible {
		return PrimaryAvailable
	}
	// A known zero is evidence of exhaustion only if all relevant windows
	// are current and measured. Unknown windows cannot authorize switching.
	found, exhausted := false, false
	for _, group := range v.ModelGroups {
		if group.Key != a.QuotaGroup && group.Key != "" {
			continue
		}
		if group.Key == a.QuotaGroup {
			if found {
				return PrimaryUnknown
			}
			found = true
		}
		if len(group.Windows) == 0 {
			return PrimaryUnknown
		}
		for _, w := range group.Windows {
			if w.Kind == "unknown" || (w.Status != string(model.UsageLive) && w.Status != string(model.UsageCached)) ||
				!(w.Remaining >= 0 && w.Remaining <= 100) || (w.ResetTime != nil && !now.Before(*w.ResetTime)) {
				return PrimaryUnknown
			}
			exhausted = exhausted || w.Remaining == 0
		}
	}
	if found && exhausted {
		return PrimaryExhausted
	}
	return PrimaryUnknown
}
