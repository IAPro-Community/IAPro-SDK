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
	expiry := now.Add(time.Minute)
	req := SwitchRequest{OwnerID: "owner", RunID: "run", Source: source, AllowSwitch: true, BeforeDispatch: true, Now: now}
	c := Account{
		OwnerID: "owner", Scope: target, Authorized: true, TermsPermit: true, ModelCompatible: true, QuotaScope: target, Model: "gpt-test", GroupMapped: true,
		ExpiresAt: now.Add(time.Minute),
		Eligibility: quota.EligibilityInput{Authenticated: true, Health: "healthy", View: quota.QuotaView{
			Provider: "codex", Profile: "b", Scope: target, ExpiresAt: &expiry, Status: "LIVE", FetchedAt: now, Available: true,
			ModelGroups: []quota.ModelGroup{{Windows: []quota.Window{{Kind: "5h", Remaining: 50, Status: "LIVE"}}}},
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
		{"wrong identity version", func(a *Account) { a.QuotaScope.IdentityVersion = "old" }},
		{"wrong account evidence", func(a *Account) { a.QuotaScope.AccountID = "other" }},
		{"unmapped model group", func(a *Account) { a.GroupMapped = false }},
		{"different model group", func(a *Account) { a.QuotaGroup = "other" }},
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
	paid := c
	paid.Paid = true
	req.AllowPaid = true
	if _, err := SelectFallback(req, []Account{paid}); !errors.Is(err, ErrNoEligibleAccount) {
		t.Fatal("paid fallback without reservation accepted")
	}
	paid.Reservation = &BudgetReservation{ID: "reservation", OwnerID: "owner", RunID: "run", Scope: target, Model: paid.Model, MaxMicroUSD: 10, ExpiresAt: expiry}
	if _, err := SelectFallback(req, []Account{paid}); err != nil {
		t.Fatalf("valid paid reservation rejected: %v", err)
	}
	paid.Reservation.RunID = "other-run"
	if _, err := SelectFallback(req, []Account{paid}); !errors.Is(err, ErrNoEligibleAccount) {
		t.Fatal("foreign reservation accepted")
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

func TestModelPoolAvailability(t *testing.T) {
	now := time.Now()
	expired := now.Add(-time.Second)
	good := quota.Window{Kind: "5h", Status: "LIVE", Remaining: 50}
	bad := quota.Window{Kind: "weekly", Status: "LIVE", Remaining: 0}
	v := quota.QuotaView{ModelGroups: []quota.ModelGroup{
		{Key: "a", Windows: []quota.Window{bad}},
		{Key: "b", Windows: []quota.Window{good}},
	}}
	if modelPoolAvailable(v, "a", now) || !modelPoolAvailable(v, "b", now) {
		t.Fatal("unrelated group must not authorize selected pool")
	}
	v.ModelGroups = append(v.ModelGroups, quota.ModelGroup{Windows: []quota.Window{bad}})
	if modelPoolAvailable(v, "b", now) {
		t.Fatal("global exhausted quota must block model")
	}
	good.ResetTime = &expired
	v.ModelGroups = []quota.ModelGroup{{Key: "b", Windows: []quota.Window{good}}}
	if modelPoolAvailable(v, "b", now) {
		t.Fatal("expired evidence must not authorize")
	}
}
