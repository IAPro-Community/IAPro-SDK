// SPDX-License-Identifier: MIT
package providers

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers/spec"
)

// EnvHook allows a product/platform to layer host-specific environment rules
// (such as sandbox path stripping or browser helpers) onto a provider's env.
type EnvHook func(baseEnv map[string]string, profileHome, profileName string) map[string]string

// AuthHook allows a product to customize login/logout or identity extraction.
type AuthHook func(ctx context.Context, account Account) (Authentication, error)

// UsageHook allows a product to supply a host-native quota probe implementation.
type UsageHook func(ctx context.Context, account Account) (model.UsageSnapshot, error)

// ProviderRegistration binds a declarative Spec, an Adapter implementation, and
// optional platform extension hooks into a single extensible registry entry.
type ProviderRegistration struct {
	Spec     spec.Spec
	Adapter  Adapter
	EnvHooks []EnvHook
}

// DetectedProvider pairs a provider ID with its detection result.
type DetectedProvider struct {
	ID        string
	Detection Detection
	Err       error
}

// Registry is an extensible, thread-safe catalog of providers and their specs.
type Registry struct {
	mu      sync.RWMutex
	specs   *spec.Registry
	entries map[string]ProviderRegistration
	order   []string
}

// NewRegistry creates an empty extensible provider Registry backed by specs
// (if specs is nil, a fresh spec.NewRegistry is created).
func NewRegistry(specs *spec.Registry) *Registry {
	if specs == nil {
		specs = spec.NewRegistry()
	}
	return &Registry{
		specs:   specs,
		entries: make(map[string]ProviderRegistration),
	}
}

// Specs returns the underlying Spec registry.
func (r *Registry) Specs() *spec.Registry {
	return r.specs
}

// Register adds a provider and its declarative Spec to the registry.
// Re-registering an existing ID updates it cleanly so products can wrap/extend
// built-in providers without touching SDK code.
func (r *Registry) Register(reg ProviderRegistration) error {
	if reg.Adapter == nil {
		return errors.New("provider adapter is required")
	}
	id := strings.ToLower(strings.TrimSpace(string(reg.Adapter.ID())))
	if id == "" {
		id = strings.ToLower(strings.TrimSpace(reg.Spec.ID))
	}
	if id == "" {
		return errors.New("provider id is required")
	}
	reg.Spec.ID = id
	if err := r.specs.Register(reg.Spec); err != nil {
		return err
	}
	stored, _ := r.specs.Get(id)
	reg.Spec = stored

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entries[id]; !exists {
		r.order = append(r.order, id)
	}
	r.entries[id] = reg
	return nil
}

// AddEnvHook attaches an environment extension hook to a registered provider.
func (r *Registry) AddEnvHook(id string, hook EnvHook) error {
	if hook == nil {
		return errors.New("env hook is required")
	}
	key := strings.ToLower(strings.TrimSpace(id))
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[key]
	if !ok {
		return fmt.Errorf("provider %q is not registered", id)
	}
	entry.EnvHooks = append(entry.EnvHooks, hook)
	r.entries[key] = entry
	return nil
}

// Get returns a registered provider entry by ID.
func (r *Registry) Get(id string) (ProviderRegistration, bool) {
	key := strings.ToLower(strings.TrimSpace(id))
	r.mu.RLock()
	entry, ok := r.entries[key]
	r.mu.RUnlock()
	if !ok {
		return ProviderRegistration{}, false
	}
	if s, sok := r.specs.Get(key); sok {
		entry.Spec = s
	}
	entry.EnvHooks = append([]EnvHook(nil), entry.EnvHooks...)
	return entry, true
}

// List returns all registered provider entries in stable registration order.
func (r *Registry) List() []ProviderRegistration {
	r.mu.RLock()
	ids := append([]string(nil), r.order...)
	r.mu.RUnlock()

	out := make([]ProviderRegistration, 0, len(ids))
	for _, id := range ids {
		if entry, ok := r.Get(id); ok {
			out = append(out, entry)
		}
	}
	return out
}

// DetectAll runs Detect across every registered adapter concurrently, bounded by
// perProviderTimeout, eliminating sequential CLI launch delays.
func (r *Registry) DetectAll(ctx context.Context, perProviderTimeout time.Duration) map[string]DetectedProvider {
	if ctx == nil {
		ctx = context.Background()
	}
	if perProviderTimeout <= 0 {
		perProviderTimeout = 4 * time.Second
	}
	entries := r.List()
	results := make(map[string]DetectedProvider, len(entries))
	if len(entries) == 0 {
		return results
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, entry := range entries {
		wg.Add(1)
		go func(reg ProviderRegistration) {
			defer wg.Done()
			dctx, cancel := context.WithTimeout(ctx, perProviderTimeout)
			det, err := reg.Adapter.Detect(dctx)
			cancel()
			mu.Lock()
			results[reg.Spec.ID] = DetectedProvider{
				ID:        reg.Spec.ID,
				Detection: det,
				Err:       err,
			}
			mu.Unlock()
		}(entry)
	}
	wg.Wait()
	return results
}

// SortedIDs returns all registered provider IDs sorted alphabetically.
func (r *Registry) SortedIDs() []string {
	entries := r.List()
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.Spec.ID)
	}
	sort.Strings(ids)
	return ids
}
