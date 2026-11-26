// SPDX-License-Identifier: MIT
package providers

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers/spec"
	"github.com/IAPro-Community/IAPro-SDK/quota"
	"github.com/IAPro-Community/IAPro-SDK/routing"
)

type mockCustomAdapter struct {
	id    model.ProviderID
	delay time.Duration
	calls *atomic.Int32
}

func (m *mockCustomAdapter) ID() model.ProviderID { return m.id }
func (m *mockCustomAdapter) Capabilities() model.Capabilities {
	return model.Capabilities{Usage: true, IsolatedRuntime: true}
}
func (m *mockCustomAdapter) Detect(ctx context.Context) (Detection, error) {
	if m.calls != nil {
		m.calls.Add(1)
	}
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return Detection{}, ctx.Err()
		case <-time.After(m.delay):
		}
	}
	return Detection{Installed: true, Version: "2.0.0"}, nil
}

func TestRegistryRejectsCredentialOverlapInHistorySyncPolicy(t *testing.T) {
	reg := NewRegistry(nil)
	err := reg.Register(ProviderRegistration{
		Spec: spec.Spec{
			ID: "bad-prov",
			HistorySyncPolicy: spec.HistorySyncPolicy{
				Mode:            spec.HistorySyncLink,
				SharedPaths:     []string{"auth.json"},
				CredentialPaths: []string{"auth.json"},
			},
		},
		Adapter: &mockCustomAdapter{id: "bad-prov"},
	})
	if err == nil {
		t.Fatal("expected registration error when shared path overlaps credential path")
	}
}

func TestRegistryExtensibilityAndParallelDetectAll(t *testing.T) {
	reg := NewRegistry(nil)
	var detectCalls atomic.Int32

	customSpec := spec.Spec{
		ID:          "custom-ai",
		DisplayName: "Custom AI",
		ShortBadge:  "CUSTOM",
		AuthKind:    spec.AuthKindAPIKey,
		ModelGroups: []spec.ModelGroupSpec{
			{Key: "fast", DisplayName: "Fast Tier", Tier: spec.GroupTierPrimary, Aliases: []string{"fast"}},
			{Key: "heavy", DisplayName: "Heavy Aux", Tier: spec.GroupTierAuxiliary, Aliases: []string{"heavy"}},
		},
		QuotaPolicy: spec.QuotaPolicy{DefaultTTL: 5 * time.Minute, ProbeTimeout: 2 * time.Second},
		HistorySyncPolicy: spec.HistorySyncPolicy{
			Mode:            spec.HistorySyncLink,
			RootSubdir:      ".custom-ai",
			SharedPaths:     []string{"sessions", "history.jsonl"},
			CredentialPaths: []string{"api-key.json"},
		},
	}

	if err := reg.Register(ProviderRegistration{
		Spec:    customSpec,
		Adapter: &mockCustomAdapter{id: "custom-ai", delay: 60 * time.Millisecond, calls: &detectCalls},
	}); err != nil {
		t.Fatalf("Register custom-ai: %v", err)
	}
	if err := reg.Register(ProviderRegistration{
		Spec:    spec.Spec{ID: "custom-two"},
		Adapter: &mockCustomAdapter{id: "custom-two", delay: 60 * time.Millisecond, calls: &detectCalls},
	}); err != nil {
		t.Fatalf("Register custom-two: %v", err)
	}

	start := time.Now()
	detected := reg.DetectAll(context.Background(), time.Second)
	elapsed := time.Since(start)
	if len(detected) != 2 || !detected["custom-ai"].Detection.Installed || !detected["custom-two"].Detection.Installed {
		t.Fatalf("unexpected DetectAll results: %+v", detected)
	}
	if elapsed >= 115*time.Millisecond {
		t.Fatalf("DetectAll ran sequentially instead of parallel: %s", elapsed)
	}

	// Prove end-to-end quota + tiering + balanced selection works for the custom provider.
	now := time.Now()
	p5, p80, a100, a0 := 5.0, 80.0, 100.0, 0.0
	loadedSpec, ok := reg.Specs().Get("custom-ai")
	if !ok {
		t.Fatal("custom-ai spec missing")
	}
	snap1 := model.UsageSnapshot{
		ProviderID: "custom-ai", ProfileID: "c1", Status: model.UsageLive, FetchedAt: now,
		Windows: []model.UsageWindow{
			{Group: "fast", Kind: "5h", RemainingPercent: &p5},
			{Group: "heavy", Kind: "5h", RemainingPercent: &a100},
		},
	}
	snap2 := model.UsageSnapshot{
		ProviderID: "custom-ai", ProfileID: "c2", Status: model.UsageLive, FetchedAt: now,
		Windows: []model.UsageWindow{
			{Group: "fast", Kind: "5h", RemainingPercent: &p80},
			{Group: "heavy", Kind: "5h", RemainingPercent: &a0},
		},
	}
	v1 := quota.BuildQuotaView(snap1, "c1", "")
	v1.ModelGroups[0].Tier = loadedSpec.GroupTierFor(v1.ModelGroups[0].Key)
	v1.ModelGroups[1].Tier = loadedSpec.GroupTierFor(v1.ModelGroups[1].Key)
	v2 := quota.BuildQuotaView(snap2, "c2", "")
	v2.ModelGroups[0].Tier = loadedSpec.GroupTierFor(v2.ModelGroups[0].Key)
	v2.ModelGroups[1].Tier = loadedSpec.GroupTierFor(v2.ModelGroups[1].Key)

	dec := routing.RankAndSelect(routing.SelectionRequest{
		ProviderID: "custom-ai",
		Balanced:   true,
		Now:        now,
		Spec:       &loadedSpec,
	}, []routing.SelectionCandidate{
		{ProviderID: "custom-ai", ProfileID: "c1", Authenticated: true, Health: "healthy", View: &v1},
		{ProviderID: "custom-ai", ProfileID: "c2", Authenticated: true, Health: "healthy", View: &v2},
	})
	if dec.Selected == nil || dec.Selected.ProfileID != "c2" {
		t.Fatalf("expected custom-ai primary tier c2 to win over auxiliary-heavy c1, got %+v", dec.Selected)
	}
}
