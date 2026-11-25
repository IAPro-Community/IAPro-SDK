// SPDX-License-Identifier: MIT

package lease

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ManagedOptions defines host policy. Waiting never authorizes expired takeover.
type ManagedOptions struct {
	TTL          time.Duration
	RenewEvery   time.Duration
	RetryEvery   time.Duration
	StoreTimeout time.Duration
	Wait         bool
}

// Guard maintains one fencing generation through dispatch and host accounting.
// A caller must always Finish, even if the dispatch context was canceled.
type Guard struct {
	coordinator *Coordinator
	key         Key
	record      Record
	options     ManagedOptions
	ctx         context.Context
	cancel      context.CancelFunc
	stop        chan struct{}
	done        chan struct{}
	renewErr    error
	finishOnce  sync.Once
	finishErr   error
}

// AcquireManaged acquires a reservation and starts its renewal loop. The
// caller's cancellation stops work but does not abandon the fence while host
// accounting is outstanding. No expired generation is reclaimed implicitly.
func (c *Coordinator) AcquireManaged(ctx context.Context, key Key, holder string, options ManagedOptions) (*Guard, error) {
	if options.TTL <= 0 || options.RenewEvery <= 0 || options.RenewEvery >= options.TTL ||
		options.StoreTimeout <= 0 || options.StoreTimeout >= options.TTL-options.RenewEvery ||
		(options.Wait && options.RetryEvery <= 0) {
		return nil, errors.New("invalid managed lease options")
	}
	var record Record
	for {
		var err error
		record, err = c.Acquire(ctx, key, holder, options.TTL, 0)
		if err == nil {
			break
		}
		if !options.Wait || !errors.Is(err, ErrBusy) {
			return nil, err
		}
		timer := time.NewTimer(options.RetryEvery)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	runCtx, cancel := context.WithCancel(ctx)
	g := &Guard{coordinator: c, key: key, record: record, options: options,
		ctx: runCtx, cancel: cancel, stop: make(chan struct{}), done: make(chan struct{})}
	go g.maintain()
	return g, nil
}

func (g *Guard) maintain() {
	defer close(g.done)
	ticker := time.NewTicker(g.options.RenewEvery)
	defer ticker.Stop()
	for {
		select {
		case <-g.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), g.options.StoreTimeout)
			_, err := g.coordinator.Renew(ctx, g.key, g.record, g.options.TTL)
			cancel()
			if err != nil {
				g.renewErr = err
				g.cancel()
				return
			}
		}
	}
}

// Context is the context to use for protected provider work.
func (g *Guard) Context() context.Context { return g.ctx }

// Fence identifies the original acquisition, unaffected by renewal revisions.
func (g *Guard) Fence() uint64 { return g.record.Fence }

// Valid rechecks the fence immediately before dispatch. It is not a substitute
// for atomic enforcement at a host-controlled write boundary.
func (g *Guard) Valid(ctx context.Context) (bool, error) {
	if err := g.ctx.Err(); err != nil {
		return false, err
	}
	return g.coordinator.Valid(ctx, g.key, g.record)
}

// Finish stops renewal exactly once. The host must prove that protected
// activity has ceased and required accounting is durable before passing true.
// Any renewal failure retains the fence, even if the host claims quiescence.
func (g *Guard) Finish(quiescent bool) error {
	g.finishOnce.Do(func() {
		close(g.stop)
		<-g.done
		g.cancel()
		if g.renewErr != nil {
			g.finishErr = errors.Join(ErrReconcile, g.renewErr)
			return
		}
		if !quiescent {
			g.finishErr = ErrReconcile
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), g.options.StoreTimeout)
		defer cancel()
		g.finishErr = g.coordinator.Release(ctx, g.key, g.record, true)
	})
	return g.finishErr
}

// ReconcileExpired releases exactly one expired generation after the host
// supplies independent evidence of quiescence. Never use time alone as proof.
func (c *Coordinator) ReconcileExpired(ctx context.Context, key Key, fence uint64, quiescent bool) error {
	if !quiescent || fence == 0 || !key.valid() {
		return ErrReconcile
	}
	record, err := c.store.Load(ctx, key)
	if err != nil {
		return err
	}
	if record.Fence != fence || record.Holder == "" || c.now().Before(record.ExpiresAt) {
		return ErrStale
	}
	return c.Release(ctx, key, record, true)
}
