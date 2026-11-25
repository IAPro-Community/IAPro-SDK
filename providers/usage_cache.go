package providers

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

var ErrIdentityMismatch = errors.New("provider account identity could not be verified")
var ErrProbeCapacity = errors.New("quota probe cache capacity reached")

// UsageCache coalesces probes per owner and verified credential generation.
// The cache is process-local; it neither stores secrets nor authorizes dispatch.
type UsageCache struct {
	mu       sync.Mutex
	slots    map[Account]*usageSlot
	ttl      time.Duration
	backoff  time.Duration
	maxSlots int
	now      func() time.Time
	auth     AuthProvider
	probe    UsageProvider
}

type usageSlot struct {
	done  chan struct{}
	value model.UsageSnapshot
	err   error
	until time.Time
}

// NewUsageCache requires an adapter that independently verifies identity,
// including IdentityVersion, on every call. TTL and capacity are host policy.
func NewUsageCache(auth AuthProvider, probe UsageProvider, ttl, backoff time.Duration, maxSlots int, now func() time.Time) (*UsageCache, error) {
	if auth == nil || probe == nil || ttl <= 0 || backoff <= 0 || maxSlots <= 0 || now == nil {
		return nil, errors.New("invalid quota cache configuration")
	}
	return &UsageCache{slots: make(map[Account]*usageSlot), ttl: ttl, backoff: backoff, maxSlots: maxSlots, now: now, auth: auth, probe: probe}, nil
}

func (c *UsageCache) verify(ctx context.Context, a Account) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	auth, err := c.auth.InspectAuth(ctx, a)
	if err != nil || !auth.Authenticated || auth.Scope != a.Scope || auth.ObservedAt.IsZero() || auth.ObservedAt.After(c.now()) || c.now().Sub(auth.ObservedAt) >= c.ttl {
		return ErrIdentityMismatch
	}
	return ctx.Err()
}

// GetUsage never returns an old observation after reauthentication or expiry.
// A canceled waiter does not cancel another caller's in-flight probe.
func (c *UsageCache) GetUsage(ctx context.Context, a Account) (model.UsageSnapshot, error) {
	if err := a.Validate(); err != nil {
		return model.UsageSnapshot{}, err
	}
	if err := c.verify(ctx, a); err != nil {
		return model.UsageSnapshot{}, err
	}
	for {
		c.mu.Lock()
		now := c.now()
		if s, ok := c.slots[a]; ok {
			if s.done != nil {
				done := s.done
				c.mu.Unlock()
				select {
				case <-ctx.Done():
					return model.UsageSnapshot{}, ctx.Err()
				case <-done:
					continue
				}
			}
			if now.Before(s.until) {
				value, err := cloneUsage(s.value), s.err
				c.mu.Unlock()
				if err != nil {
					return model.UsageSnapshot{}, err
				}
				if err := c.verify(ctx, a); err != nil {
					return model.UsageSnapshot{}, err
				}
				if value.Status == model.UsageLive {
					value.Status = model.UsageCached
				}
				return value, nil
			}
			delete(c.slots, a)
		}
		for key, slot := range c.slots {
			if slot.done == nil && !now.Before(slot.until) {
				delete(c.slots, key)
			}
		}
		if len(c.slots) >= c.maxSlots {
			c.mu.Unlock()
			return model.UsageSnapshot{}, ErrProbeCapacity
		}
		s := &usageSlot{done: make(chan struct{})}
		c.slots[a] = s
		c.mu.Unlock()

		value, err := c.probe.GetUsage(ctx, a)
		if err == nil {
			err = c.verify(ctx, a)
		}
		now = c.now()
		if err == nil && (value.AccountScope != a.Scope || value.ProviderID != a.Scope.ProviderID || value.ProfileID != a.Scope.ProfileID ||
			value.FetchedAt.IsZero() || value.FetchedAt.After(now)) {
			err = ErrIdentityMismatch
		}
		until := value.FetchedAt.Add(c.ttl)
		if value.ExpiresAt != nil && value.ExpiresAt.Before(until) {
			until = *value.ExpiresAt
		}
		if err == nil && !now.Before(until) {
			err = errors.New("quota observation already expired")
		}
		if err != nil {
			value = model.UsageSnapshot{}
			until = now.Add(c.backoff)
		} else {
			value.ExpiresAt = &until
		}
		c.mu.Lock()
		s.value, s.err, s.until = cloneUsage(value), err, until
		close(s.done)
		s.done = nil
		if ctx.Err() != nil {
			delete(c.slots, a)
		}
		c.mu.Unlock()
		return cloneUsage(value), err
	}
}

func copyPtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneUsage(v model.UsageSnapshot) model.UsageSnapshot {
	v.ExpiresAt = copyPtr(v.ExpiresAt)
	v.Windows = append([]model.UsageWindow(nil), v.Windows...)
	for i := range v.Windows {
		w := &v.Windows[i]
		w.Limit = copyPtr(w.Limit)
		w.Used = copyPtr(w.Used)
		w.Remaining = copyPtr(w.Remaining)
		w.UsedPercent = copyPtr(w.UsedPercent)
		w.RemainingPercent = copyPtr(w.RemainingPercent)
		w.ResetTime = copyPtr(w.ResetTime)
	}
	return v
}
