package lease

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentAcquisitionAndFencing(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	store := &MemoryStore{}
	a, _ := New(store, func() time.Time { return now })
	b, _ := New(store, func() time.Time { return now })
	key := Key{OwnerID: "owner", Provider: "codex", AccountID: "account"}
	var wins atomic.Int32
	var winner Record
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			record, err := a.Acquire(ctx, key, "process-a", time.Minute, 0)
			if err == nil {
				wins.Add(1)
				mu.Lock()
				winner = record
				mu.Unlock()
			} else if !errors.Is(err, ErrBusy) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("acquisitions = %d", wins.Load())
	}
	now = now.Add(time.Minute)
	if _, err := b.Acquire(ctx, key, "process-b", time.Minute, 0); !errors.Is(err, ErrReconcile) {
		t.Fatalf("expiry alone must not permit takeover: %v", err)
	}
	replacement, err := b.Acquire(ctx, key, "process-b", time.Minute, winner.Fence)
	if err != nil || replacement.Fence != winner.Fence+1 {
		t.Fatalf("takeover: %+v %v", replacement, err)
	}
	if err := a.Release(ctx, key, winner, true); !errors.Is(err, ErrStale) {
		t.Fatal("stale holder released newer generation")
	}
	if valid, err := b.Valid(ctx, key, replacement); err != nil || !valid {
		t.Fatal("replacement lease not valid")
	}
	if err := b.Release(ctx, key, replacement, true); err != nil {
		t.Fatal(err)
	}
	next, err := a.Acquire(ctx, key, "process-c", time.Minute, 0)
	if err != nil || next.Fence != replacement.Fence+1 {
		t.Fatal("release reset fence")
	}
}
