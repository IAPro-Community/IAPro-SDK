// SPDX-License-Identifier: MIT
package routing

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers/spec"
	"github.com/IAPro-Community/IAPro-SDK/quota"
)

const defaultTieEpsilon = 0.5

// SelectionCandidate describes one candidate account for automatic or :balanced selection.
type SelectionCandidate struct {
	Scope         model.AccountScope
	ProviderID    string
	ProfileID     string
	Account       string
	Plan          string
	Authenticated bool
	Disabled      bool
	Excluded      bool
	InCooldown    bool
	Health        string
	Priority      int
	IsDefault     bool
	IsBound       bool
	Snapshot      model.UsageSnapshot
	View          *quota.QuotaView
}

// SelectionRequest configures tier-aware ranking and optional balanced cursor rotation.
type SelectionRequest struct {
	ProviderID     string
	TargetModel    string
	Balanced       bool
	LastSelectedID string
	TieEpsilon     float64
	Now            time.Time
	Policy         spec.QuotaPolicy
	Spec           *spec.Spec
}

// CandidateOutcome reports eligibility, tier, capacity and rejection diagnostics for one candidate.
type CandidateOutcome struct {
	ProfileID      string                  `json:"profile_id"`
	Scope          model.AccountScope      `json:"scope"`
	Eligible       bool                    `json:"eligible"`
	Reason         quota.EligibilityReason `json:"reason"`
	RejectReason   string                  `json:"reject_reason,omitempty"`
	ActiveTier     spec.GroupTier          `json:"active_tier,omitempty"`
	Remaining      float64                 `json:"remaining"`
	Score          float64                 `json:"score"`
	NeedsRefresh   bool                    `json:"needs_refresh,omitempty"`
	Projection     quota.QuotaProjection   `json:"projection"`
	ScoreBreakdown []string                `json:"score_breakdown,omitempty"`
}

// SelectionDecision is the deterministic outcome of RankAndSelect.
type SelectionDecision struct {
	Selected              *SelectionCandidate  `json:"selected,omitempty"`
	ActiveTier            spec.GroupTier       `json:"active_tier,omitempty"`
	UsedAuxiliaryFallback bool                 `json:"used_auxiliary_fallback,omitempty"`
	ReasonCode            string               `json:"reason_code"`
	Explanation           string               `json:"explanation"`
	NeedsRefresh          []model.AccountScope `json:"needs_refresh,omitempty"`
	Evaluations           []CandidateOutcome   `json:"evaluations"`
}

// RankAndSelect evaluates candidates under the provider's tiering and freshness policy.
//
// Tier rule:
//  1. Candidates with usable primary quota (>0%) always outrank auxiliary-only candidates.
//  2. Within primary tier, higher primary remaining always wins regardless of auxiliary balance.
//  3. Auxiliary pools (e.g. Claude inside AGY) are selected automatically only when every
//     eligible candidate has 0% primary remaining (UsedAuxiliaryFallback=true).
//  4. If TargetModel resolves to a specific group key, only that group is scored.
func RankAndSelect(req SelectionRequest, candidates []SelectionCandidate) SelectionDecision {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	eps := req.TieEpsilon
	if eps <= 0 {
		eps = defaultTieEpsilon
	}
	providerID := strings.ToLower(strings.TrimSpace(req.ProviderID))

	var resolvedSpec spec.Spec
	hasSpec := false
	if req.Spec != nil {
		resolvedSpec = *req.Spec
		hasSpec = true
	} else if s, ok := spec.Get(providerID); ok {
		resolvedSpec = s
		hasSpec = true
	}

	ttl := req.Policy.DefaultTTL
	if ttl <= 0 && hasSpec {
		ttl = resolvedSpec.QuotaPolicy.DefaultTTL
	}
	require5h := req.Policy.Requires5hForBalanced
	if !require5h && hasSpec {
		require5h = resolvedSpec.QuotaPolicy.Requires5hForBalanced
	}

	targetGroup := ""
	hasTargetGroup := false
	if hasSpec && strings.TrimSpace(req.TargetModel) != "" {
		targetGroup, hasTargetGroup = resolvedSpec.ResolveGroupForModel(req.TargetModel)
	}

	outcomes := make([]CandidateOutcome, 0, len(candidates))
	var needsRefresh []model.AccountScope

	type rankedEntry struct {
		idx     int
		cand    *SelectionCandidate
		outcome *CandidateOutcome
	}
	var primaryTier []rankedEntry
	var auxiliaryTier []rankedEntry

	for i := range candidates {
		c := &candidates[i]
		if providerID != "" && !strings.EqualFold(strings.TrimSpace(c.ProviderID), providerID) {
			continue
		}
		var view quota.QuotaView
		if c.View != nil {
			view = *c.View
		} else {
			view = quota.BuildQuotaView(c.Snapshot, c.Account, c.Plan)
		}
		view.ComputeAvailabilityAt(now)
		proj := view.ProjectionAt(now, ttl, c.Authenticated, c.Health)

		out := CandidateOutcome{
			ProfileID:  c.ProfileID,
			Scope:      c.Scope,
			Projection: proj,
		}

		if c.Disabled {
			out.Reason = quota.ReasonUnavailable
			out.RejectReason = "profile disabled"
			outcomes = append(outcomes, out)
			continue
		}
		if c.Excluded {
			out.Reason = quota.ReasonUnavailable
			out.RejectReason = "excluded from current cycle"
			outcomes = append(outcomes, out)
			continue
		}

		elig := quota.EvaluateEligibility(quota.EligibilityInput{
			View:              view,
			Authenticated:     c.Authenticated,
			Health:            c.Health,
			InCooldown:        c.InCooldown,
			ExplicitSelection: false,
			Now:               now,
			FreshnessTTL:      ttl,
			Require5hWindows:  req.Balanced && require5h,
		})
		out.Reason = elig.Reason

		if !elig.Eligible {
			out.RejectReason = elig.Message
			if c.Authenticated && !c.InCooldown && (elig.Reason == quota.ReasonStaleQuota || elig.Reason == quota.ReasonUnknownQuota || proj.NeedsResetReprobe) {
				out.NeedsRefresh = true
				needsRefresh = append(needsRefresh, refreshScopeFor(*c, providerID))
			}
			outcomes = append(outcomes, out)
			continue
		}

		if hasTargetGroup {
			rem, ok := view.GroupRemainingForKeyAt(targetGroup, now)
			if !ok || rem <= 0 {
				out.Reason = quota.ReasonExhausted
				out.RejectReason = fmt.Sprintf("target model group %q exhausted", targetGroup)
				outcomes = append(outcomes, out)
				continue
			}
			tier := spec.GroupTierPrimary
			if hasSpec {
				tier = resolvedSpec.GroupTierFor(targetGroup)
			}
			out.Eligible = true
			out.ActiveTier = tier
			out.Remaining = rem
			out.Score = scoreForCandidate(rem, c, req.Balanced)
			out.ScoreBreakdown = append(out.ScoreBreakdown, fmt.Sprintf("%s %.0f%%", targetGroup, rem))
			idx := len(outcomes)
			outcomes = append(outcomes, out)
			primaryTier = append(primaryTier, rankedEntry{idx: idx, cand: c, outcome: &outcomes[idx]})
			continue
		}

		primRem, primOK := view.PrimaryGroupRemainingAt(now)
		auxRem, auxOK := view.AuxiliaryGroupRemainingAt(now)

		if primOK && primRem > 0 {
			out.Eligible = true
			out.ActiveTier = spec.GroupTierPrimary
			out.Remaining = primRem
			out.Score = scoreForCandidate(primRem, c, req.Balanced)
			if auxOK && auxRem > 0 && !req.Balanced {
				out.Score += auxRem * 0.001
			}
			out.ScoreBreakdown = append(out.ScoreBreakdown, fmt.Sprintf("primary %.0f%%", primRem))
			idx := len(outcomes)
			outcomes = append(outcomes, out)
			primaryTier = append(primaryTier, rankedEntry{idx: idx, cand: c, outcome: &outcomes[idx]})
			continue
		}

		if auxOK && auxRem > 0 {
			out.Eligible = true
			out.ActiveTier = spec.GroupTierAuxiliary
			out.Remaining = auxRem
			out.Score = scoreForCandidate(auxRem, c, req.Balanced)
			out.ScoreBreakdown = append(out.ScoreBreakdown, fmt.Sprintf("auxiliary %.0f%%", auxRem))
			idx := len(outcomes)
			outcomes = append(outcomes, out)
			auxiliaryTier = append(auxiliaryTier, rankedEntry{idx: idx, cand: c, outcome: &outcomes[idx]})
			continue
		}

		out.Reason = quota.ReasonExhausted
		out.RejectReason = "all quota pools exhausted"
		outcomes = append(outcomes, out)
	}

	var pool []rankedEntry
	usedAux := false
	activeTier := spec.GroupTierPrimary

	switch {
	case len(primaryTier) > 0:
		pool = primaryTier
	case len(auxiliaryTier) > 0:
		pool = auxiliaryTier
		usedAux = true
		activeTier = spec.GroupTierAuxiliary
	default:
		return SelectionDecision{
			ReasonCode:   "NO_ELIGIBLE_CANDIDATES",
			Explanation:  buildNoEligibleExplanation(outcomes),
			NeedsRefresh: needsRefresh,
			Evaluations:  outcomes,
		}
	}

	sort.SliceStable(pool, func(i, j int) bool {
		if math.Abs(pool[i].outcome.Score-pool[j].outcome.Score) > 0.0001 {
			return pool[i].outcome.Score > pool[j].outcome.Score
		}
		return pool[i].cand.ProfileID < pool[j].cand.ProfileID
	})

	leaderCount := 1
	topScore := pool[0].outcome.Score
	for i := 1; i < len(pool); i++ {
		if topScore-pool[i].outcome.Score <= eps {
			leaderCount++
		} else {
			break
		}
	}
	leaders := pool[:leaderCount]

	selectedIdx := 0
	if req.Balanced && leaderCount > 1 && strings.TrimSpace(req.LastSelectedID) != "" {
		for i, entry := range leaders {
			if entry.cand.ProfileID == req.LastSelectedID {
				selectedIdx = (i + 1) % leaderCount
				break
			}
		}
	}

	winner := leaders[selectedIdx]
	reasonCode := "SELECTED_PRIMARY_HIGHEST"
	explanation := fmt.Sprintf("selected %s with %.0f%% primary quota", winner.cand.ProfileID, winner.outcome.Remaining)
	if usedAux {
		reasonCode = "SELECTED_AUXILIARY_FALLBACK"
		explanation = fmt.Sprintf("all primary quotas exhausted; selected %s via auxiliary quota (%.0f%%)", winner.cand.ProfileID, winner.outcome.Remaining)
	} else if req.Balanced && leaderCount > 1 {
		reasonCode = "SELECTED_BALANCED_ROTATION"
		explanation = fmt.Sprintf("balanced rotation among %d equally available accounts -> %s (%.0f%%)", leaderCount, winner.cand.ProfileID, winner.outcome.Remaining)
	} else if req.Balanced {
		reasonCode = "SELECTED_BALANCED_HIGHEST"
		explanation = fmt.Sprintf("balanced selected highest remaining %s (%.0f%%)", winner.cand.ProfileID, winner.outcome.Remaining)
	}

	return SelectionDecision{
		Selected:              winner.cand,
		ActiveTier:            activeTier,
		UsedAuxiliaryFallback: usedAux,
		ReasonCode:            reasonCode,
		Explanation:           explanation,
		NeedsRefresh:          needsRefresh,
		Evaluations:           outcomes,
	}
}

func scoreForCandidate(remaining float64, c *SelectionCandidate, balanced bool) float64 {
	score := remaining
	if !balanced {
		if c.IsBound {
			score += 5.0
		} else if c.IsDefault {
			score += 2.0
		}
		if c.Priority > 0 {
			score += float64(maxInt(0, 10-c.Priority)) * 0.1
		}
	}
	return score
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func refreshScopeFor(c SelectionCandidate, fallbackProvider string) model.AccountScope {
	if c.Scope.ProviderID != "" || c.Scope.ProfileID != "" {
		return c.Scope
	}
	p := c.ProviderID
	if p == "" {
		p = fallbackProvider
	}
	return model.AccountScope{
		ProviderID: p,
		ProfileID:  c.ProfileID,
	}
}

func buildNoEligibleExplanation(outcomes []CandidateOutcome) string {
	if len(outcomes) == 0 {
		return "no candidate profiles configured"
	}
	parts := make([]string, 0, len(outcomes))
	for _, out := range outcomes {
		msg := out.RejectReason
		if msg == "" {
			msg = string(out.Reason)
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", out.ProfileID, msg))
	}
	return "no eligible profiles available: " + strings.Join(parts, ", ")
}
