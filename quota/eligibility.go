// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE for source and copyright.

package quota

import (
	"math"
	"strings"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

// EligibilityReason is a stable machine-readable gate outcome.
type EligibilityReason string

const (
	ReasonOK               EligibilityReason = "OK"
	ReasonUnknownQuota     EligibilityReason = "UNKNOWN_QUOTA"
	ReasonStaleQuota       EligibilityReason = "STALE_QUOTA"
	ReasonUnsupportedQuota EligibilityReason = "UNSUPPORTED_QUOTA"
	ReasonExhausted        EligibilityReason = "EXHAUSTED"
	ReasonRateLimited      EligibilityReason = "RATE_LIMITED"
	ReasonAuthRequired     EligibilityReason = "AUTH_REQUIRED"
	ReasonUnavailable      EligibilityReason = "UNAVAILABLE"
	ReasonNetworkError     EligibilityReason = "NETWORK_ERROR"
	ReasonCooldown         EligibilityReason = "COOLDOWN"
	ReasonNotAuthenticated EligibilityReason = "NOT_AUTHENTICATED"
)

// Eligibility is the single routing gate shared by CLI scheduler, Mission
// RecommendResources, runtime routing and manual validation.
type Eligibility struct {
	Eligible   bool              `json:"eligible"`
	Reason     EligibilityReason `json:"reason"`
	Confidence string            `json:"confidence"`
	Message    string            `json:"message,omitempty"`
}

// EligibilityInput carries the facts needed to evaluate a provider account.
type EligibilityInput struct {
	View              QuotaView
	Authenticated     bool
	Health            string
	NetworkError      bool
	InCooldown        bool
	ExplicitSelection bool
	Now               time.Time
	FreshnessTTL      time.Duration
	Require5hWindows  bool
}

// EvaluateEligibility applies hard gates in a single canonical order.
func EvaluateEligibility(in EligibilityInput) Eligibility {
	confidence := confidenceOf(in.View.Status)
	out := Eligibility{Confidence: confidence}

	if !in.Authenticated {
		out.Reason = ReasonNotAuthenticated
		out.Message = "Conta não autenticada"
		return out
	}
	if in.NetworkError {
		out.Reason = ReasonNetworkError
		out.Message = "Erro de rede ao consultar o provedor"
		return out
	}

	health := strings.ToLower(strings.TrimSpace(in.Health))
	switch health {
	case strings.ToLower(string(model.HealthAuthRequired)), "auth_required":
		out.Reason = ReasonAuthRequired
		out.Message = "Autenticação necessária"
		return out
	case strings.ToLower(string(model.HealthRateLimited)):
		out.Reason = ReasonRateLimited
		out.Message = "Conta rate-limited"
		return out
	case strings.ToLower(string(model.HealthUnavailable)), "unavailable", "unhealthy":
		out.Reason = ReasonUnavailable
		out.Message = "Provedor indisponível"
		return out
	}

	if in.InCooldown {
		out.Reason = ReasonCooldown
		out.Message = "Conta em cooldown de rate limit"
		return out
	}

	status := strings.ToUpper(strings.TrimSpace(in.View.Status))
	if status == string(model.UsageRateLimited) || in.View.AvailReasons.RateLimited {
		out.Reason = ReasonRateLimited
		out.Confidence = "RATE_LIMITED"
		out.Message = "Conta rate-limited"
		return out
	}

	if status == string(model.UsageError) || in.View.AvailReasons.AuthRequired {
		out.Reason = ReasonUnavailable
		out.Message = "Quota evidence unavailable"
		return out
	}

	// QuotaView.Available is the aggregate authority. ExhaustedWindows may
	// contain partial diagnostics while another independent model group still
	// has capacity (for example AGY Gemini exhausted, Claude/GPT available).
	// Reject as exhausted only when no group remains available.
	if !in.View.Available {
		if status == string(model.UsageUnknown) || in.View.AvailReasons.UnknownQuota {
			out.Reason = ReasonUnknownQuota
			out.Confidence = "UNKNOWN"
			out.Message = "Quota UNKNOWN — inelegível para alocação automática"
			if in.ExplicitSelection {
				// Still reject when Available=false; explicit only relaxes
				// UNKNOWN-but-available accounts.
				if in.View.Available {
					out.Eligible = true
					out.Message = "Quota não aferida; seleção explícita permitida"
				}
			}
			return out
		}
		out.Reason = ReasonExhausted
		out.Message = "resource unavailable"
		return out
	}

	if status == string(model.UsageUnknown) || in.View.AvailReasons.UnknownQuota {
		out.Reason = ReasonUnknownQuota
		out.Confidence = "UNKNOWN"
		out.Message = "Quota UNKNOWN — inelegível para alocação automática"
		if in.ExplicitSelection {
			out.Eligible = true
			out.Message = "Quota não aferida; seleção explícita permitida"
		}
		return out
	}
	if status == string(model.UsageUnsupported) {
		out.Reason = ReasonUnsupportedQuota
		out.Confidence = "UNSUPPORTED"
		out.Message = "Quota não suportada pelo provedor"
		if in.ExplicitSelection {
			out.Eligible = true
		}
		return out
	}

	if !in.ExplicitSelection && status != string(model.UsageLive) && status != string(model.UsageCached) {
		out.Reason = ReasonStaleQuota
		out.Message = "Quota evidence is not current or measured"
		return out
	}

	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	if !in.ExplicitSelection && in.FreshnessTTL > 0 {
		if in.View.FetchedAt.IsZero() || now.Sub(in.View.FetchedAt) > in.FreshnessTTL || now.Sub(in.View.FetchedAt) < -time.Minute {
			out.Reason = ReasonStaleQuota
			out.Message = "Quota observation is stale and needs refresh"
			return out
		}
		if in.View.NeedsResetReprobeAt(now) {
			out.Reason = ReasonStaleQuota
			out.Message = "Quota window has rolled over and needs refresh"
			return out
		}
	}
	if !in.ExplicitSelection && in.Require5hWindows {
		if !HasValidFiveHourWindows(in.View, now) {
			out.Reason = ReasonUnknownQuota
			out.Message = "5h quota evidence is missing for one or more model groups"
			return out
		}
	}

	out.Eligible = true
	out.Reason = ReasonOK
	return out
}

// HasValidFiveHourWindows ensures every observed model group exposes a valid
// short-window (5h) measurement so weekly-only readings never masquerade as
// full multi-group readiness.
func HasValidFiveHourWindows(view QuotaView, now time.Time) bool {
	if len(view.ModelGroups) == 0 {
		return false
	}
	for _, g := range view.ModelGroups {
		has5h := false
		for _, w := range g.Windows {
			kind := strings.ToLower(strings.TrimSpace(w.Kind))
			if kind != "5h" && kind != "five_hour" && kind != "claude_5h" && kind != "claude_five_hour" {
				continue
			}
			if windowExpired(w, now) {
				continue
			}
			if !math.IsNaN(w.Remaining) && !math.IsInf(w.Remaining, 0) && w.Remaining >= 0 && w.Remaining <= 100 {
				has5h = true
				break
			}
		}
		if !has5h {
			return false
		}
	}
	return true
}

func confidenceOf(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case string(model.UsageLive):
		return "LIVE"
	case string(model.UsageCached):
		return "CACHED"
	case string(model.UsageEstimated):
		return "ESTIMATED"
	case string(model.UsageUnsupported):
		return "UNSUPPORTED"
	case string(model.UsageRateLimited):
		return "RATE_LIMITED"
	default:
		return "UNKNOWN"
	}
}
