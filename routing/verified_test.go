// SPDX-License-Identifier: MIT

package routing

import (
	"testing"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

func TestVerifiedAccountAndPrimaryDistinguishUnknownFromExhausted(t *testing.T) {
	now := time.Now()
	expiry := now.Add(time.Minute)
	remaining := 75.0
	scope := model.AccountScope{ProviderID: "codex", ProfileID: "profile", AccountID: "account", IdentityVersion: "generation"}
	in := EvidenceInput{OwnerID: "owner", Scope: scope, Model: "gpt-test", Now: now,
		Authorized: true, TermsPermit: true, Authenticated: true, Health: string(model.HealthHealthy),
		Snapshot: model.UsageSnapshot{ProviderID: "codex", ProfileID: scope.ProfileID, AccountScope: scope,
			ModelName: "gpt-test", Status: model.UsageLive, Source: model.SourceOfficialAPI,
			Confidence: model.UsageConfidenceHigh, FetchedAt: now, ExpiresAt: &expiry,
			Windows: []model.UsageWindow{{Kind: "5h", Group: "claude_gpt", RemainingPercent: &remaining}}}}
	a, err := VerifiedAccount(in)
	if err != nil || AssessPrimary(a, now) != PrimaryAvailable {
		t.Fatalf("verified primary rejected: %v", err)
	}
	remaining = 0
	a, err = VerifiedAccount(in)
	if err != nil || AssessPrimary(a, now) != PrimaryExhausted {
		t.Fatalf("measured zero not distinguished: %v", err)
	}
	in.Snapshot.Windows[0].RemainingPercent = nil
	if _, err := VerifiedAccount(in); err == nil {
		t.Fatal("unknown quota authorized")
	}
	in.Snapshot.Windows[0].RemainingPercent = &remaining
	in.Model = "gpt-other"
	if _, err := VerifiedAccount(in); err == nil {
		t.Fatal("model family guessed from alias")
	}
	in.Model = "gpt-test"
	in.Snapshot.AccountScope.IdentityVersion = "old"
	if _, err := VerifiedAccount(in); err == nil {
		t.Fatal("stale credential evidence accepted")
	}
}
