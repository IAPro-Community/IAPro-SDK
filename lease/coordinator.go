// SPDX-License-Identifier: MIT

// Package lease coordinates admission to an account-scoped resource. A shared,
// durable Store and host-side fencing are required for interprocess guarantees.
package lease

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

var (
	ErrBusy       = errors.New("account resource is already leased")
	ErrReconcile  = errors.New("expired holder must be reconciled before takeover")
	ErrStale      = errors.New("stale lease")
	ErrContention = errors.New("lease store contention")
)

// Key must use a canonical owner and provider account identity shared by all
// participating products. Profile aliases and credential rotations must not
// create a new resource key for the same physical account.
type Key struct {
	OwnerID   string
	Provider  string
	AccountID string
}

func (k Key) valid() bool {
	return k.OwnerID != "" && k.Provider != "" && k.AccountID != ""
}

// Record persists the last fencing token even after release. Revision is a
// CAS version; Fence is monotonic per resource and changes on acquisition.
type Record struct {
	Holder    string
	Fence     uint64
	Revision  uint64
	ExpiresAt time.Time
}

// Store must implement linearizable CAS and durable monotonic versions for
// shared use. An absent record is represented by Record{}; never delete it.
type Store interface {
	Load(context.Context, Key) (Record, error)
	CompareAndSwap(context.Context, Key, uint64, Record) (bool, error)
}

type Coordinator struct {
	store Store
	now   func() time.Time
}

func New(store Store, now func() time.Time) (*Coordinator, error) {
	if store == nil || now == nil {
		return nil, errors.New("lease store and clock required")
	}
	return &Coordinator{store: store, now: now}, nil
}

// Acquire does not reclaim an expired record based on time alone: a host
// must first prove the exact prior fencing generation is stopped/reconciled.
// A lease is admission evidence, not a substitute for process termination.
func (c *Coordinator) Acquire(ctx context.Context, key Key, holder string, ttl time.Duration, reconciledFence uint64) (Record, error) {
	if !key.valid() || holder == "" || ttl <= 0 {
		return Record{}, errors.New("invalid lease request")
	}
	for i := 0; i < 8; i++ {
		if err := ctx.Err(); err != nil {
			return Record{}, err
		}
		old, err := c.store.Load(ctx, key)
		if err != nil {
			return Record{}, err
		}
		now := c.now()
		if now.IsZero() || old.Fence == math.MaxUint64 || old.Revision == math.MaxUint64 {
			return Record{}, ErrStale
		}
		if old.Holder != "" {
			if now.Before(old.ExpiresAt) {
				return Record{}, ErrBusy
			}
			if reconciledFence == 0 || old.Fence != reconciledFence {
				return Record{}, ErrReconcile
			}
		}
		next := Record{Holder: holder, Fence: old.Fence + 1, Revision: old.Revision + 1, ExpiresAt: now.Add(ttl)}
		ok, err := c.store.CompareAndSwap(ctx, key, old.Revision, next)
		if err != nil {
			return Record{}, err
		}
		if ok {
			return next, nil
		}
	}
	return Record{}, ErrContention
}

func (c *Coordinator) Renew(ctx context.Context, key Key, lease Record, ttl time.Duration) (Record, error) {
	if ttl <= 0 || !key.valid() {
		return Record{}, ErrStale
	}
	return c.change(ctx, key, lease, false, ttl)
}

// Release is allowed only after the host confirms the resource is quiescent.
// A stale holder cannot release a later generation.
func (c *Coordinator) Release(ctx context.Context, key Key, lease Record, quiescent bool) error {
	if !quiescent || !key.valid() {
		return ErrReconcile
	}
	_, err := c.change(ctx, key, lease, true, 0)
	return err
}

func (c *Coordinator) change(ctx context.Context, key Key, lease Record, release bool, ttl time.Duration) (Record, error) {
	if lease.Holder == "" || lease.Fence == 0 {
		return Record{}, ErrStale
	}
	for i := 0; i < 8; i++ {
		if err := ctx.Err(); err != nil {
			return Record{}, err
		}
		old, err := c.store.Load(ctx, key)
		if err != nil {
			return Record{}, err
		}
		now := c.now()
		if old.Holder != lease.Holder || old.Fence != lease.Fence || old.Revision == math.MaxUint64 || now.IsZero() ||
			(!release && !now.Before(old.ExpiresAt)) {
			return Record{}, ErrStale
		}
		next := old
		next.Revision++
		if release {
			next.Holder, next.ExpiresAt = "", time.Time{}
		} else {
			next.ExpiresAt = now.Add(ttl)
		}
		ok, err := c.store.CompareAndSwap(ctx, key, old.Revision, next)
		if err != nil {
			return Record{}, err
		}
		if ok {
			return next, nil
		}
	}
	return Record{}, ErrContention
}

// Valid checks current fencing evidence immediately before a protected host
// operation. Callers must enforce the fence atomically at the write boundary.
func (c *Coordinator) Valid(ctx context.Context, key Key, lease Record) (bool, error) {
	if !key.valid() || lease.Holder == "" || lease.Fence == 0 {
		return false, nil
	}
	old, err := c.store.Load(ctx, key)
	if err != nil {
		return false, err
	}
	return old.Holder == lease.Holder && old.Fence == lease.Fence && c.now().Before(old.ExpiresAt), nil
}

// MemoryStore is a process-local reference implementation and test fixture.
// It is deliberately not advertised as a cross-process or crash-safe store.
type MemoryStore struct {
	mu      sync.Mutex
	records map[Key]Record
}

func (s *MemoryStore) Load(ctx context.Context, key Key) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	return s.records[key], nil
}

func (s *MemoryStore) CompareAndSwap(ctx context.Context, key Key, revision uint64, next Record) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if s.records[key].Revision != revision {
		return false, nil
	}
	if revision == math.MaxUint64 || next.Revision != revision+1 || next.Fence < s.records[key].Fence {
		return false, ErrStale
	}
	if s.records == nil {
		s.records = make(map[Key]Record)
	}
	s.records[key] = next
	return true, nil
}
