package providers

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// Bindings is the shared typed registry for host-specific ports. A binding
// registers a capability; registration alone does not authorize its use.
type Bindings[T any] struct {
	mu      sync.RWMutex
	entries map[string]T
}

func NewBindings[T any]() *Bindings[T] {
	return &Bindings[T]{entries: make(map[string]T)}
}

func NormalizeID(id string) string { return strings.ToLower(strings.TrimSpace(id)) }

func (b *Bindings[T]) Register(id string, value T) error {
	id = NormalizeID(id)
	if id == "" {
		return errors.New("provider ID required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.entries[id]; ok {
		return errors.New("provider already registered: " + id)
	}
	b.entries[id] = value
	return nil
}

func (b *Bindings[T]) Get(id string) (T, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	v, ok := b.entries[NormalizeID(id)]
	return v, ok
}

func (b *Bindings[T]) Unregister(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, NormalizeID(id))
}

type Binding[T any] struct {
	ID    string
	Value T
}

func (b *Bindings[T]) List() []Binding[T] {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Binding[T], 0, len(b.entries))
	for id, v := range b.entries {
		out = append(out, Binding[T]{ID: id, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// DetectBindings executes bounded concurrent discovery using an explicit host
// callback. Detectors must honor their context and never perform login or install.
func DetectBindings[T, D any](ctx context.Context, bindings []Binding[T], timeout time.Duration, parallelism int, detect func(context.Context, T) D) map[string]D {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	if parallelism < 1 {
		parallelism = 4
	}
	out := make(map[string]D, len(bindings))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallelism)
	for _, entry := range bindings {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return out
		}
		wg.Add(1)
		go func(e Binding[T]) {
			defer wg.Done()
			defer func() { <-sem }()
			callCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			v := detect(callCtx, e.Value)
			mu.Lock()
			out[e.ID] = v
			mu.Unlock()
		}(entry)
	}
	wg.Wait()
	return out
}
