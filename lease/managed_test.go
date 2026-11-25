// SPDX-License-Identifier: MIT

package lease

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func managedOptions() ManagedOptions {
	return ManagedOptions{TTL: time.Minute, RenewEvery: time.Second, RetryEvery: time.Millisecond, StoreTimeout: time.Second, Wait: true}
}

func TestManagedLeaseDoesNotReclaimAmbiguousGeneration(t *testing.T) {
	store := &MemoryStore{}
	now := time.Now()
	c, _ := New(store, func() time.Time { return now })
	key := Key{OwnerID: "owner", Provider: "codex", AccountID: "account"}
	g, err := c.AcquireManaged(context.Background(), key, "first", managedOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Finish(false); !errors.Is(err, ErrReconcile) {
		t.Fatalf("ambiguous finish: %v", err)
	}
	if err := g.Finish(true); !errors.Is(err, ErrReconcile) {
		t.Fatal("second finish overrode initial decision")
	}
	now = now.Add(2 * time.Minute)
	if _, err := c.AcquireManaged(context.Background(), key, "second", managedOptions()); !errors.Is(err, ErrReconcile) {
		t.Fatalf("expired lease taken over: %v", err)
	}
	if err := c.ReconcileExpired(context.Background(), key, g.Fence(), false); !errors.Is(err, ErrReconcile) {
		t.Fatal("reconciliation without evidence allowed")
	}
	if err := c.ReconcileExpired(context.Background(), key, g.Fence()+1, true); !errors.Is(err, ErrStale) {
		t.Fatal("wrong fence reconciled")
	}
	if err := c.ReconcileExpired(context.Background(), key, g.Fence(), true); err != nil {
		t.Fatal(err)
	}
	next, err := c.AcquireManaged(context.Background(), key, "second", managedOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer next.Finish(true)
	if next.Fence() <= g.Fence() {
		t.Fatal("fence did not advance")
	}
}

type renewalFailureStore struct {
	MemoryStore
	fail atomic.Bool
}

func (s *renewalFailureStore) CompareAndSwap(ctx context.Context, key Key, revision uint64, next Record) (bool, error) {
	if s.fail.Load() {
		return false, errors.New("fixture store unavailable")
	}
	return s.MemoryStore.CompareAndSwap(ctx, key, revision, next)
}

func TestManagedRenewalFailureCancelsWorkAndRetainsFence(t *testing.T) {
	store := &renewalFailureStore{}
	c, _ := New(store, time.Now)
	key := Key{OwnerID: "owner", Provider: "codex", AccountID: "account"}
	opts := managedOptions()
	opts.RenewEvery = time.Millisecond
	g, err := c.AcquireManaged(context.Background(), key, "holder", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Finish(false)
	store.fail.Store(true)
	select {
	case <-g.Context().Done():
	case <-time.After(5 * time.Second):
		t.Fatal("work was not canceled")
	}
	store.fail.Store(false)
	if err := g.Finish(true); !errors.Is(err, ErrReconcile) {
		t.Fatalf("failed renewal released lease: %v", err)
	}
	r, err := store.Load(context.Background(), key)
	if err != nil || r.Holder != "holder" {
		t.Fatal("ambiguous fence lost")
	}
}

func TestManagedWaitIsCancelableAndReleaseIgnoresCallerCancellation(t *testing.T) {
	c, _ := New(&MemoryStore{}, time.Now)
	key := Key{OwnerID: "owner", Provider: "codex", AccountID: "account"}
	ctx, cancel := context.WithCancel(context.Background())
	g, err := c.AcquireManaged(ctx, key, "first", managedOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer g.Finish(true)
	waitCtx, stop := context.WithCancel(context.Background())
	stop()
	if _, err := c.AcquireManaged(waitCtx, key, "waiter", managedOptions()); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled waiter acquired lease")
	}
	cancel()
	if err := g.Finish(true); err != nil {
		t.Fatal(err)
	}
	next, err := c.AcquireManaged(context.Background(), key, "second", managedOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer next.Finish(true)
	if next.Fence() <= g.Fence() {
		t.Fatal("fence did not advance after release")
	}
}
