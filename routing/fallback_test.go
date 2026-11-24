package routing

import (
	"errors"
	"testing"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/quota"
)

func TestSelectFallback(t *testing.T) {
	now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	source := model.AccountScope{ProviderID: "codex", ProfileID: "a", AccountID: "1", IdentityVersion: "v1"}
	target := model.AccountScope{ProviderID: "codex", ProfileID: "b", AccountID: "2", IdentityVersion: "v1"}
	req := SwitchRequest{OwnerID: "owner", Source: source, AllowSwitch: true, BeforeDispatch: true, Now: now}
	c := Account{
		OwnerID: "owner", Scope: target, Authorized: true, TermsPermit: true, ModelCompatible: true,
		ExpiresAt: now.Add(time.Minute),
		Eligibility: quota.EligibilityInput{Authenticated: true, Health: "healthy", View: quota.QuotaView{
			Provider: "codex", Profile: "b", Status: "LIVE", FetchedAt: now, Available: true,
			ModelGroups: []quota.ModelGroup{{Windows: []quota.Window{{Kind: "5h", Remaining: 50}}}},
		}},
	}
	if got, err := SelectFallback(req, []Account{c}); err != nil || got.Scope != target {
		t.Fatalf("eligible account rejected: %+v %v", got, err)
	}
	for _, tc := range []struct {
		name string
		edit func(*Account)
	}{
		{"other owner", func(a *Account) { a.OwnerID = "other" }},
		{"same identity", func(a *Account) { a.Scope.AccountID = "1" }},
		{"paid", func(a *Account) { a.Paid = true }},
		{"terms", func(a *Account) { a.TermsPermit = false }},
		{"unauthorized", func(a *Account) { a.Authorized = false }},
		{"incompatible model", func(a *Account) { a.ModelCompatible = false }},
		{"expired", func(a *Account) { a.ExpiresAt = now }},
		{"unknown", func(a *Account) { a.Eligibility.View.Status = "UNKNOWN" }},
		{"cooldown", func(a *Account) { a.Eligibility.InCooldown = true }},
		{"wrong profile evidence", func(a *Account) { a.Eligibility.View.Profile = "a" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := c
			tc.edit(&x)
			if _, err := SelectFallback(req, []Account{x}); !errors.Is(err, ErrNoEligibleAccount) {
				t.Fatalf("expected refusal, got %v", err)
			}
		})
	}
	req.BeforeDispatch = false
	if _, err := SelectFallback(req, []Account{c}); !errors.Is(err, ErrUnsafeSwitch) {
		t.Fatalf("ambiguous execution must not retry: %v", err)
	}
	req.Reconciled, req.NoOutput, req.NoSideEffects = true, true, true
	if _, err := SelectFallback(req, []Account{c}); err != nil {
		t.Fatal(err)
	}
}
