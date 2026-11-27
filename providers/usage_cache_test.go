package providers

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

type fixtureProvider struct {
	now   time.Time
	calls atomic.Int32
	bad   atomic.Bool
}

func (p *fixtureProvider) InspectAuth(_ context.Context, a Account) (Authentication, error) {
	return Authentication{Scope: a.Scope, Authenticated: !p.bad.Load(), ObservedAt: p.now}, nil
}

func (p *fixtureProvider) GetUsage(_ context.Context, a Account) (model.UsageSnapshot, error) {
	p.calls.Add(1)
	remaining := 50.0
	return model.UsageSnapshot{ProviderID: a.Scope.ProviderID, ProfileID: a.Scope.ProfileID, AccountScope: a.Scope, FetchedAt: p.now, Status: model.UsageLive,
		Windows: []model.UsageWindow{{Kind: "5h", RemainingPercent: &remaining}}}, nil
}

func TestUsageCacheIsolationAndCoalescing(t *testing.T) {
	p := &fixtureProvider{now: time.Now()}
	cache, err := NewUsageCache(p, p, time.Minute, time.Second, 10, func() time.Time { return p.now })
	if err != nil {
		t.Fatal(err)
	}
	a := Account{OwnerID: "owner", Scope: model.AccountScope{ProviderID: "codex", ProfileID: "a", AccountID: "1", IdentityVersion: "v1"}}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := cache.GetUsage(context.Background(), a)
			if err != nil {
				t.Error(err)
				return
			}
			*v.Windows[0].RemainingPercent = 0
		}()
	}
	wg.Wait()
	v, err := cache.GetUsage(context.Background(), a)
	if err != nil || p.calls.Load() != 1 || *v.Windows[0].RemainingPercent != 50 || v.Status != model.UsageCached {
		t.Fatalf("cache result: %+v %v calls=%d", v, err, p.calls.Load())
	}
	a.Scope.IdentityVersion = "v2"
	if _, err := cache.GetUsage(context.Background(), a); err != nil || p.calls.Load() != 2 {
		t.Fatalf("new credential generation must probe: %v", err)
	}
	p.bad.Store(true)
	if _, err := cache.GetUsage(context.Background(), a); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatal("logout must invalidate cached access")
	}
}

type expiredProvider struct {
	fixtureProvider
}

func (p *expiredProvider) GetUsage(_ context.Context, _ Account) (model.UsageSnapshot, error) {
	p.calls.Add(1)
	return model.UsageSnapshot{}, &ProbeError{Code: ProbeAuthExpired, Message: "private diagnostic"}
}

func TestUsageCachePreservesAuthExpiredWithoutRawData(t *testing.T) {
	p := &expiredProvider{fixtureProvider: fixtureProvider{now: time.Now()}}
	c, err := NewUsageCache(p, p, time.Minute, time.Second, 4, func() time.Time { return p.now })
	if err != nil {
		t.Fatal(err)
	}
	a := Account{OwnerID: "owner", Scope: model.AccountScope{ProviderID: "codex", ProfileID: "p", AccountID: "a", IdentityVersion: "v1"}}
	for i := 0; i < 2; i++ {
		snap, err := c.GetUsage(context.Background(), a)
		if ClassifyProbeError(err) != ProbeAuthExpired || err.Error() != string(ProbeAuthExpired) || len(snap.Windows) != 0 {
			t.Fatalf("unsanitized or lost auth diagnostic: %v", err)
		}
	}
	if p.calls.Load() != 1 {
		t.Fatalf("backoff failed: %d calls", p.calls.Load())
	}
}

func TestExplicitRefreshRespectsCoalescingAndCredentialGeneration(t *testing.T) {
	p := &fixtureProvider{now: time.Now()}
	c, err := NewUsageCache(p, p, time.Minute, time.Second, 8, func() time.Time { return p.now })
	if err != nil {
		t.Fatal(err)
	}
	a := Account{OwnerID: "owner", Scope: model.AccountScope{ProviderID: "codex", ProfileID: "p", AccountID: "a", IdentityVersion: "v1"}}
	if _, err := c.GetUsage(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RefreshUsage(context.Background(), a, time.Second); err != nil || p.calls.Load() != 1 {
		t.Fatalf("recent probe should coalesce: %v, %d", err, p.calls.Load())
	}
	p.now = p.now.Add(2 * time.Second)
	if _, err := c.RefreshUsage(context.Background(), a, time.Second); err != nil || p.calls.Load() != 2 {
		t.Fatalf("explicit refresh should probe: %v, %d", err, p.calls.Load())
	}
	a.Scope.IdentityVersion = "v2"
	if _, err := c.GetUsage(context.Background(), a); err != nil || p.calls.Load() != 3 {
		t.Fatalf("changed credential generation should probe: %v, %d", err, p.calls.Load())
	}
}
