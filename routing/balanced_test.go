// SPDX-License-Identifier: MIT
package routing

import (
	"testing"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers/spec"
	"github.com/IAPro-Community/IAPro-SDK/quota"
)

func agySnapshot(profile string, gemini5h, geminiWeek, claude5h, claudeWeek float64, fetchedAt time.Time) model.UsageSnapshot {
	return model.UsageSnapshot{
		ProviderID: "agy",
		ProfileID:  profile,
		Status:     model.UsageLive,
		Source:     model.SourceCLIOutput,
		FetchedAt:  fetchedAt,
		Windows: []model.UsageWindow{
			{Group: "gemini", Kind: "5h", RemainingPercent: &gemini5h},
			{Group: "gemini", Kind: "weekly", RemainingPercent: &geminiWeek},
			{Group: "claude_gpt", Kind: "claude_5h", RemainingPercent: &claude5h},
			{Group: "claude_gpt", Kind: "claude_weekly", RemainingPercent: &claudeWeek},
		},
	}
}

func TestRankAndSelectAGYPrimaryBeatsAuxiliary100(t *testing.T) {
	now := time.Now()
	candidates := []SelectionCandidate{
		{
			Scope:         model.AccountScope{ProviderID: "agy", ProfileID: "low-gemini-high-claude", AccountID: "a1", IdentityVersion: "1"},
			ProviderID:    "agy",
			ProfileID:     "low-gemini-high-claude",
			Authenticated: true,
			Health:        "healthy",
			Snapshot:      agySnapshot("low-gemini-high-claude", 5, 80, 100, 100, now),
		},
		{
			Scope:         model.AccountScope{ProviderID: "agy", ProfileID: "high-gemini-zero-claude", AccountID: "a2", IdentityVersion: "1"},
			ProviderID:    "agy",
			ProfileID:     "high-gemini-zero-claude",
			Authenticated: true,
			Health:        "healthy",
			Snapshot:      agySnapshot("high-gemini-zero-claude", 90, 95, 0, 0, now),
		},
	}

	decision := RankAndSelect(SelectionRequest{
		ProviderID: "agy",
		Balanced:   true,
		Now:        now,
	}, candidates)

	if decision.Selected == nil || decision.Selected.ProfileID != "high-gemini-zero-claude" {
		t.Fatalf("expected high-gemini-zero-claude to win, got %+v (%s)", decision.Selected, decision.Explanation)
	}
	if decision.UsedAuxiliaryFallback {
		t.Fatalf("must not mark UsedAuxiliaryFallback when primary quota is available")
	}
	if decision.ActiveTier != spec.GroupTierPrimary {
		t.Fatalf("expected primary tier, got %s", decision.ActiveTier)
	}
}

func TestRankAndSelectAGYFallsBackToAuxiliaryOnlyWhenAllPrimaryExhausted(t *testing.T) {
	now := time.Now()
	candidates := []SelectionCandidate{
		{
			Scope:         model.AccountScope{ProviderID: "agy", ProfileID: "zero-gemini-mid-claude", AccountID: "a1", IdentityVersion: "1"},
			ProviderID:    "agy",
			ProfileID:     "zero-gemini-mid-claude",
			Authenticated: true,
			Health:        "healthy",
			Snapshot:      agySnapshot("zero-gemini-mid-claude", 0, 80, 40, 100, now),
		},
		{
			Scope:         model.AccountScope{ProviderID: "agy", ProfileID: "zero-gemini-full-claude", AccountID: "a2", IdentityVersion: "1"},
			ProviderID:    "agy",
			ProfileID:     "zero-gemini-full-claude",
			Authenticated: true,
			Health:        "healthy",
			Snapshot:      agySnapshot("zero-gemini-full-claude", 0, 90, 100, 100, now),
		},
	}

	decision := RankAndSelect(SelectionRequest{
		ProviderID: "agy",
		Balanced:   true,
		Now:        now,
	}, candidates)

	if decision.Selected == nil || decision.Selected.ProfileID != "zero-gemini-full-claude" {
		t.Fatalf("expected zero-gemini-full-claude via auxiliary fallback, got %+v", decision.Selected)
	}
	if !decision.UsedAuxiliaryFallback || decision.ActiveTier != spec.GroupTierAuxiliary {
		t.Fatalf("expected auxiliary fallback flag and tier, got aux=%t tier=%s", decision.UsedAuxiliaryFallback, decision.ActiveTier)
	}
}

func TestRankAndSelectExplicitAuxiliaryModelTargetsClaudePool(t *testing.T) {
	now := time.Now()
	candidates := []SelectionCandidate{
		{
			ProviderID:    "agy",
			ProfileID:     "acc1",
			Authenticated: true,
			Health:        "healthy",
			Snapshot:      agySnapshot("acc1", 100, 100, 10, 100, now),
		},
		{
			ProviderID:    "agy",
			ProfileID:     "acc2",
			Authenticated: true,
			Health:        "healthy",
			Snapshot:      agySnapshot("acc2", 0, 0, 85, 90, now),
		},
	}

	decision := RankAndSelect(SelectionRequest{
		ProviderID:  "agy",
		TargetModel: "claude-sonnet-4",
		Balanced:    true,
		Now:         now,
	}, candidates)

	if decision.Selected == nil || decision.Selected.ProfileID != "acc2" {
		t.Fatalf("explicit claude model should select acc2 with 85%% claude pool, got %+v", decision.Selected)
	}
}

func TestRankAndSelectFlagsStaleAccountsForRefreshEvenWhenAnotherAccountIsFresh(t *testing.T) {
	now := time.Now()
	staleAt := now.Add(-20 * time.Minute)
	candidates := []SelectionCandidate{
		{
			Scope:         model.AccountScope{ProviderID: "agy", ProfileID: "fresh-low", AccountID: "a1", IdentityVersion: "1"},
			ProviderID:    "agy",
			ProfileID:     "fresh-low",
			Authenticated: true,
			Health:        "healthy",
			Snapshot:      agySnapshot("fresh-low", 20, 80, 100, 100, now),
		},
		{
			Scope:         model.AccountScope{ProviderID: "agy", ProfileID: "stale-high", AccountID: "a2", IdentityVersion: "1"},
			ProviderID:    "agy",
			ProfileID:     "stale-high",
			Authenticated: true,
			Health:        "healthy",
			Snapshot:      agySnapshot("stale-high", 100, 100, 100, 100, staleAt),
		},
	}

	decision := RankAndSelect(SelectionRequest{
		ProviderID: "agy",
		Balanced:   true,
		Now:        now,
	}, candidates)

	if len(decision.NeedsRefresh) != 1 || decision.NeedsRefresh[0].ProfileID != "stale-high" {
		t.Fatalf("expected stale-high in NeedsRefresh even though fresh-low is eligible, got %+v", decision.NeedsRefresh)
	}
	if decision.Selected == nil || decision.Selected.ProfileID != "fresh-low" {
		t.Fatalf("expected fresh-low as interim eligible before refresh, got %+v", decision.Selected)
	}
}

func TestRankAndSelectBalancedRotatesOnlyAmongTopTies(t *testing.T) {
	now := time.Now()
	candidates := []SelectionCandidate{
		{ProviderID: "codex", ProfileID: "a", Authenticated: true, Health: "healthy", Snapshot: agySnapshot("a", 90, 90, 0, 0, now)},
		{ProviderID: "codex", ProfileID: "b", Authenticated: true, Health: "healthy", Snapshot: agySnapshot("b", 90, 90, 0, 0, now)},
		{ProviderID: "codex", ProfileID: "c", Authenticated: true, Health: "healthy", Snapshot: agySnapshot("c", 50, 90, 0, 0, now)},
	}
	for i := range candidates {
		candidates[i].Snapshot.ProviderID = "codex"
		candidates[i].Snapshot.Windows = candidates[i].Snapshot.Windows[:2]
		candidates[i].Snapshot.Windows[0].Group = ""
		candidates[i].Snapshot.Windows[1].Group = ""
	}

	d1 := RankAndSelect(SelectionRequest{ProviderID: "codex", Balanced: true, LastSelectedID: "a", Now: now}, candidates)
	if d1.Selected == nil || d1.Selected.ProfileID != "b" {
		t.Fatalf("expected rotation to b after a, got %+v", d1.Selected)
	}
	d2 := RankAndSelect(SelectionRequest{ProviderID: "codex", Balanced: true, LastSelectedID: "b", Now: now}, candidates)
	if d2.Selected == nil || d2.Selected.ProfileID != "a" {
		t.Fatalf("expected rotation back to a (never c at 50%%), got %+v", d2.Selected)
	}
	_ = quota.ReasonOK
}

func TestRankAndSelectDoesNotGuessUnknownModelGroup(t *testing.T) {
	decision := RankAndSelect(SelectionRequest{ProviderID: "codex", TargetModel: "unmapped-model", Now: time.Now()}, nil)
	if decision.Selected != nil || decision.ReasonCode != "UNKNOWN_MODEL_GROUP" {
		t.Fatal("unknown model must not select an unrelated pool")
	}
}
