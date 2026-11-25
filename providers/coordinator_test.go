// SPDX-License-Identifier: MIT
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

func TestUsageCoordinatorRunsBatchInParallelWithinBudget(t *testing.T) {
	var current, maxConcurrent atomic.Int32
	coord, err := NewDirectCoordinator(func(ctx context.Context, a Account) (model.UsageSnapshot, error) {
		n := current.Add(1)
		for {
			prev := maxConcurrent.Load()
			if n <= prev || maxConcurrent.CompareAndSwap(prev, n) {
				break
			}
		}
		defer current.Add(-1)

		switch a.Scope.ProfileID {
		case "slow-timeout":
			select {
			case <-ctx.Done():
				return model.UsageSnapshot{}, ctx.Err()
			case <-time.After(2 * time.Second):
				return model.UsageSnapshot{}, nil
			}
		case "locked":
			return model.UsageSnapshot{}, &ProbeError{Code: ProbeProfileLocked, Message: "TUI holds lock"}
		case "expired":
			return model.UsageSnapshot{}, ErrIdentityMismatch
		default:
			time.Sleep(40 * time.Millisecond)
			rem := 85.0
			return model.UsageSnapshot{
				ProviderID:   a.Scope.ProviderID,
				ProfileID:    a.Scope.ProfileID,
				AccountScope: a.Scope,
				Status:       model.UsageLive,
				FetchedAt:    time.Now(),
				Windows:      []model.UsageWindow{{Kind: "5h", RemainingPercent: &rem}},
			}, nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	accounts := []Account{
		{OwnerID: "u1", Scope: model.AccountScope{ProviderID: "agy", ProfileID: "ok1", AccountID: "1", IdentityVersion: "1"}},
		{OwnerID: "u1", Scope: model.AccountScope{ProviderID: "agy", ProfileID: "ok2", AccountID: "2", IdentityVersion: "1"}},
		{OwnerID: "u1", Scope: model.AccountScope{ProviderID: "agy", ProfileID: "locked", AccountID: "3", IdentityVersion: "1"}},
		{OwnerID: "u1", Scope: model.AccountScope{ProviderID: "agy", ProfileID: "expired", AccountID: "4", IdentityVersion: "1"}},
		{OwnerID: "u1", Scope: model.AccountScope{ProviderID: "agy", ProfileID: "slow-timeout", AccountID: "5", IdentityVersion: "1"}},
	}

	start := time.Now()
	outcomes := coord.RefreshBatch(context.Background(), accounts, BatchOptions{
		MaxConcurrency:  4,
		PerProbeTimeout: 120 * time.Millisecond,
		GlobalTimeout:   500 * time.Millisecond,
	})
	elapsed := time.Since(start)

	if len(outcomes) != len(accounts) {
		t.Fatalf("got %d outcomes, want %d", len(outcomes), len(accounts))
	}
	if maxConcurrent.Load() < 2 {
		t.Fatalf("expected parallel execution, maxConcurrent=%d", maxConcurrent.Load())
	}
	if elapsed > 450*time.Millisecond {
		t.Fatalf("parallel batch took too long: %s", elapsed)
	}
	if outcomes[0].Code != ProbeOK || outcomes[2].Code != ProbeProfileLocked || outcomes[3].Code != ProbeAuthExpired || outcomes[4].Code != ProbeTimeout {
		t.Fatalf("unexpected diagnostic codes: %+v", outcomes)
	}
}

func TestUsageCoordinatorCoalescesDuplicateAccountRequests(t *testing.T) {
	var calls sync.Mutex
	count := 0
	coord, err := NewDirectCoordinator(func(ctx context.Context, a Account) (model.UsageSnapshot, error) {
		calls.Lock()
		count++
		calls.Unlock()
		time.Sleep(30 * time.Millisecond)
		return model.UsageSnapshot{ProviderID: a.Scope.ProviderID, ProfileID: a.Scope.ProfileID, AccountScope: a.Scope, Status: model.UsageLive, FetchedAt: time.Now()}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	acc := Account{OwnerID: "u1", Scope: model.AccountScope{ProviderID: "agy", ProfileID: "dup", AccountID: "1", IdentityVersion: "1"}}
	outcomes := coord.RefreshBatch(context.Background(), []Account{acc, acc, acc}, BatchOptions{MaxConcurrency: 4})
	if len(outcomes) != 3 {
		t.Fatalf("got %d outcomes", len(outcomes))
	}
	if count != 1 {
		t.Fatalf("expected single coalesced probe call, got %d", count)
	}
	if errors.Is(outcomes[0].Err, context.DeadlineExceeded) {
		t.Fatal("unexpected timeout")
	}
}
