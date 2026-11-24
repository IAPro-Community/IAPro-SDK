// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE.

package routing

import (
	"strings"

	"github.com/IAPro-Community/IAPro-SDK/classifier"
	"github.com/IAPro-Community/IAPro-SDK/model"
)

// RoutingSwitchReason is the real typed cause for a resource switch.
// Empty means no switch (selected == requested and no failover).
type RoutingSwitchReason string

const (
	SwitchReasonNone                 RoutingSwitchReason = ""
	SwitchReasonPreferredUnavailable RoutingSwitchReason = "preferred_unavailable"
	SwitchReasonQuotaExhausted       RoutingSwitchReason = "quota_exhausted"
	SwitchReasonRateLimited          RoutingSwitchReason = "rate_limited"
	SwitchReasonCooldown             RoutingSwitchReason = "cooldown"
	SwitchReasonUnauthenticated      RoutingSwitchReason = "unauthenticated"
	SwitchReasonUnhealthy            RoutingSwitchReason = "unhealthy"
	SwitchReasonModelIncapable       RoutingSwitchReason = "model_incapable"
)

// DeriveRoutingSwitchReason maps a human reason + failure evidence to the
// typed switch cause. Typed failure kinds win; free prose is classified
// through internal/core/classifier (the only place allowed to match failure
// text for quota/rate-limit — provider guardrails enforce this). Unknown
// prose falls back to preferred_unavailable only when a fallback happened.
func DeriveRoutingSwitchReason(fallback bool, reason string, failureKind model.FailureKind, failureMessage string) RoutingSwitchReason {
	if !fallback {
		return SwitchReasonNone
	}
	// Typed failure evidence first — never report everything as rate limit.
	switch failureKind {
	case model.FailureQuota:
		return SwitchReasonQuotaExhausted
	case model.FailureRateLimit:
		if mentionsCooldown(reason + " " + failureMessage) {
			return SwitchReasonCooldown
		}
		return SwitchReasonRateLimited
	case model.FailureAuth:
		return SwitchReasonUnauthenticated
	case model.FailureNetwork, model.FailureProvider:
		return SwitchReasonUnhealthy
	}
	prose := reason + " " + failureMessage
	switch classifier.Classify(nil, prose).Kind {
	case model.FailureQuota:
		return SwitchReasonQuotaExhausted
	case model.FailureRateLimit:
		if mentionsCooldown(prose) {
			return SwitchReasonCooldown
		}
		return SwitchReasonRateLimited
	case model.FailureAuth:
		return SwitchReasonUnauthenticated
	case model.FailureNetwork, model.FailureProvider:
		return SwitchReasonUnhealthy
	}
	lower := strings.ToLower(prose)
	switch {
	case mentionsCooldown(prose):
		return SwitchReasonCooldown
	case strings.Contains(lower, "incapable") || (strings.Contains(lower, "model") && strings.Contains(lower, "fallback")):
		return SwitchReasonModelIncapable
	case strings.Contains(lower, "unhealthy") || (strings.Contains(lower, "unavailable") && !strings.Contains(lower, "preferred")) || strings.Contains(lower, "health"):
		return SwitchReasonUnhealthy
	case strings.Contains(lower, "preferred"):
		return SwitchReasonPreferredUnavailable
	case strings.Contains(lower, "model"):
		return SwitchReasonModelIncapable
	default:
		return SwitchReasonPreferredUnavailable
	}
}

// mentionsCooldown is the only inline failure-text match allowed here:
// "cooldown" is not a quota/rate-limit substring, so the provider substring
// guardrail does not apply to it.
func mentionsCooldown(prose string) bool {
	return strings.Contains(strings.ToLower(prose), "cooldown")
}
