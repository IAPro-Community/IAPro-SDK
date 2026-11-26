// SPDX-License-Identifier: MIT
package quota

import (
	"testing"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers/spec"
)

func TestAGYTieringPrimaryAlwaysBeatsAuxiliaryOnly(t *testing.T) {
	now := time.Now()
	g5, g90, c100, c0 := 5.0, 90.0, 100.0, 0.0

	accLowGeminiHighClaude := BuildQuotaView(model.UsageSnapshot{
		ProviderID: "agy",
		ProfileID:  "p1",
		Status:     model.UsageLive,
		FetchedAt:  now,
		Windows: []model.UsageWindow{
			{Group: "gemini", Kind: "5h", RemainingPercent: &g5},
			{Group: "claude_gpt", Kind: "claude_5h", RemainingPercent: &c100},
		},
	}, "p1@example.com", "")

	accHighGeminiZeroClaude := BuildQuotaView(model.UsageSnapshot{
		ProviderID: "agy",
		ProfileID:  "p2",
		Status:     model.UsageLive,
		FetchedAt:  now,
		Windows: []model.UsageWindow{
			{Group: "gemini", Kind: "5h", RemainingPercent: &g90},
			{Group: "claude_gpt", Kind: "claude_5h", RemainingPercent: &c0},
		},
	}, "p2@example.com", "")

	accZeroGeminiFullClaude := BuildQuotaView(model.UsageSnapshot{
		ProviderID: "agy",
		ProfileID:  "p3",
		Status:     model.UsageLive,
		FetchedAt:  now,
		Windows: []model.UsageWindow{
			{Group: "gemini", Kind: "5h", RemainingPercent: &c0},
			{Group: "claude_gpt", Kind: "claude_5h", RemainingPercent: &c100},
		},
	}, "p3@example.com", "")

	if accHighGeminiZeroClaude.CompareCapacityAt(&accLowGeminiHighClaude, now) <= 0 {
		t.Fatalf("expected Gemini 90%% + Claude 0%% to outrank Gemini 5%% + Claude 100%%")
	}
	if accLowGeminiHighClaude.CompareCapacityAt(&accZeroGeminiFullClaude, now) <= 0 {
		t.Fatalf("expected Gemini 5%% + Claude 100%% to outrank Gemini 0%% + Claude 100%% (auxiliary only)")
	}

	projAux := accZeroGeminiFullClaude.ProjectionAt(now, 5*time.Minute, true, "healthy")
	if !projAux.Available || projAux.PrimaryAvailable || !projAux.AuxiliaryOnly || projAux.ActiveTier != spec.GroupTierAuxiliary {
		t.Fatalf("unexpected auxiliary projection: %+v", projAux)
	}
}

func TestStaleAndRolloverReprobeDetection(t *testing.T) {
	now := time.Now()
	pastReset := now.Add(-2 * time.Minute)
	zero := 0.0

	v := BuildQuotaView(model.UsageSnapshot{
		ProviderID:     "agy",
		ProfileID:      "p1",
		Status:         model.UsageLive,
		FetchedAt:      now.Add(-10 * time.Minute),
		DiagnosticCode: "PROFILE_LOCKED",
		Error:          "Codex TUI active",
		Windows: []model.UsageWindow{
			{Group: "gemini", Kind: "5h", RemainingPercent: &zero, ResetTime: &pastReset},
		},
	}, "p1@example.com", "")

	if !v.NeedsResetReprobeAt(now) {
		t.Fatalf("expected NeedsResetReprobeAt=true when ResetTime is in the past")
	}
	elig := EvaluateEligibility(EligibilityInput{
		View:          v,
		Authenticated: true,
		Health:        "healthy",
		Now:           now,
		FreshnessTTL:  5 * time.Minute,
	})
	if elig.Eligible {
		t.Fatalf("expired/stale snapshot must not be eligible: %+v", elig)
	}
	proj := v.ProjectionAt(now, 5*time.Minute, true, "healthy")
	if proj.DiagnosticCode != "PROFILE_LOCKED" || proj.DiagnosticMessage != "Codex TUI active" {
		t.Fatalf("expected diagnostic preserved in projection, got %+v", proj)
	}
}
