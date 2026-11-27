// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE.

package spec

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/runtime"
)

// QuotaSource describes how a provider obtains attributable usage data.
type QuotaSource string
type IsolationGuarantee string

// GroupTier classifies a model quota family as primary or auxiliary.
// Auxiliary pools (such as Claude/GPT inside AGY) are only selected
// automatically after every eligible primary quota pool is exhausted.
type GroupTier string

const (
	GroupTierPrimary   GroupTier = "primary"
	GroupTierAuxiliary GroupTier = "auxiliary"
)

// ModelGroupSpec declares a provider's model-family quota pool.
type ModelGroupSpec struct {
	Key             string    `json:"key"`
	DisplayName     string    `json:"display_name"`
	Tier            GroupTier `json:"tier"`
	Aliases         []string  `json:"aliases,omitempty"`
	RequiredWindows []string  `json:"required_windows,omitempty"`
}

// AuthKind declares the authentication mechanism expected when adding accounts.
type AuthKind string

const (
	AuthKindOAuthCLI    AuthKind = "oauth_cli"
	AuthKindAPIKey      AuthKind = "api_key"
	AuthKindHostSession AuthKind = "host_session"
)

// QuotaPolicy governs freshness, probe budgets, and balanced rotation preconditions.
type QuotaPolicy struct {
	DefaultTTL            time.Duration `json:"default_ttl"`
	ProbeTimeout          time.Duration `json:"probe_timeout"`
	Requires5hForBalanced bool          `json:"requires_5h_for_balanced"`
}

// HistorySyncMode declares how conversation histories synchronize across accounts.
type HistorySyncMode string

const (
	HistorySyncNone            HistorySyncMode = "none"
	HistorySyncLink            HistorySyncMode = "link"
	HistorySyncIncrementalCopy HistorySyncMode = "incremental_copy"
)

// HistorySyncPolicy declares shareable history paths and forbidden credential paths.
type HistorySyncPolicy struct {
	Mode            HistorySyncMode `json:"mode"`
	RootSubdir      string          `json:"root_subdir,omitempty"`
	SharedPaths     []string        `json:"shared_paths,omitempty"`
	CredentialPaths []string        `json:"credential_paths,omitempty"`
}

const (
	IsolationUnverified              IsolationGuarantee = "UNVERIFIED"
	IsolationWorkspaceWriteNoNetwork IsolationGuarantee = "WORKSPACE_WRITE_NO_NETWORK"
	IsolationReadOnlyNoNetwork       IsolationGuarantee = "READ_ONLY_NO_NETWORK"

	QuotaLiveHTTP    QuotaSource = "LIVE_HTTP"
	QuotaLiveRPC     QuotaSource = "LIVE_RPC"
	QuotaLiveCLI     QuotaSource = "LIVE_CLI"
	QuotaUnsupported QuotaSource = "UNSUPPORTED"
)

// FlagTable maps canonical Nexus aliases to provider-native flags.
type FlagTable map[string]CanonicalAlias

// Spec is the single source of truth for a provider's binary, env, flags and
// mission launch contracts. Adapters, drivers and mission execution must derive
// from Spec instead of maintaining parallel tables.
type Spec struct {
	ID                  string
	BinaryCandidates    []string
	EnvKeys             func(profileHome, binDir, profileName string) map[string]string
	EnvDelete           []string
	Flags               FlagTable
	Kickoff             func(prompt string) []string
	Resume              func(sessionID string) []string
	AutoApproval        []string
	ReviewMode          []string
	AutonomousIsolation IsolationGuarantee
	ReviewIsolation     IsolationGuarantee
	// ClearLine is the provider-native control sequence used before a prompt
	// promoted from an interactive terminal is replaced by Nexus' banner.
	ClearLine         string
	Quota             QuotaSource
	KnownVersions     []string
	DisplayName       string
	ShortBadge        string
	InstallGuideURL   string
	AuthKind          AuthKind
	ModelGroups       []ModelGroupSpec
	QuotaPolicy       QuotaPolicy
	HistorySyncPolicy HistorySyncPolicy
}

// Registry is a thread-safe provider specification registry supporting
// built-in providers and product-specific extensions.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]Spec
	order   []string
}

// NewRegistry returns a Registry preloaded with the built-in specs.
func NewRegistry() *Registry {
	r := &Registry{
		entries: make(map[string]Spec, len(builtinOrder)),
		order:   make([]string, 0, len(builtinOrder)),
	}
	for _, id := range builtinOrder {
		r.entries[id] = cloneSpec(builtinSpecs[id])
		r.order = append(r.order, id)
	}
	return r
}

var builtinSpecs = map[string]Spec{
	"agy":      agySpec,
	"claude":   claudeSpec,
	"codex":    codexSpec,
	"cursor":   cursorSpec,
	"gemini":   geminiSpec,
	"opencode": opencodeSpec,
	"zcode":    ZCodeSpec(),
}

var builtinOrder = []string{"agy", "claude", "codex", "cursor", "gemini", "opencode", "zcode"}

var defaultRegistry = NewRegistry()

// Register validates and registers a provider specification on the default registry.
func Register(s Spec) error {
	return defaultRegistry.Register(s)
}

// Extend updates an existing specification in place on the default registry.
func Extend(id string, fn func(*Spec)) error {
	return defaultRegistry.Extend(id, fn)
}

// Unregister removes a specification by ID from the default registry (primarily for tests).
func Unregister(id string) {
	defaultRegistry.Unregister(id)
}

// Register validates and adds or replaces a provider specification.
func (r *Registry) Register(s Spec) error {
	id := strings.ToLower(strings.TrimSpace(s.ID))
	if id == "" {
		return errors.New("provider spec id is required")
	}
	s.ID = id
	if err := validateHistorySyncPolicy(s.HistorySyncPolicy); err != nil {
		return fmt.Errorf("provider %s: %w", id, err)
	}
	cloned := cloneSpec(s)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entries[id]; !exists {
		r.order = append(r.order, id)
	}
	r.entries[id] = cloned
	return nil
}

// Extend clones the existing spec for id, applies fn, validates, and saves it.
func (r *Registry) Extend(id string, fn func(*Spec)) error {
	if fn == nil {
		return errors.New("extend callback is required")
	}
	id = strings.ToLower(strings.TrimSpace(id))
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, exists := r.entries[id]
	if !exists {
		return fmt.Errorf("unknown provider spec %q", id)
	}
	updated := cloneSpec(cur)
	fn(&updated)
	updated.ID = id
	if err := validateHistorySyncPolicy(updated.HistorySyncPolicy); err != nil {
		return fmt.Errorf("provider %s: %w", id, err)
	}
	r.entries[id] = cloneSpec(updated)
	return nil
}

// Unregister removes a provider specification by ID.
func (r *Registry) Unregister(id string) {
	id = strings.ToLower(strings.TrimSpace(id))
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entries[id]; !exists {
		return
	}
	delete(r.entries, id)
	filtered := r.order[:0]
	for _, existing := range r.order {
		if existing != id {
			filtered = append(filtered, existing)
		}
	}
	r.order = filtered
}

func validateHistorySyncPolicy(p HistorySyncPolicy) error {
	if len(p.SharedPaths) == 0 || len(p.CredentialPaths) == 0 {
		return nil
	}
	for _, shared := range p.SharedPaths {
		cleanShared := filepath.ToSlash(filepath.Clean(strings.TrimSpace(shared)))
		if cleanShared == "" || cleanShared == "." || strings.HasPrefix(cleanShared, "..") {
			return fmt.Errorf("invalid shared path %q", shared)
		}
		for _, cred := range p.CredentialPaths {
			cleanCred := filepath.ToSlash(filepath.Clean(strings.TrimSpace(cred)))
			if cleanCred == "" || cleanCred == "." {
				continue
			}
			if cleanShared == cleanCred ||
				strings.HasPrefix(cleanShared, cleanCred+"/") ||
				strings.HasPrefix(cleanCred, cleanShared+"/") {
				return fmt.Errorf("shared path %q overlaps credential path %q", shared, cred)
			}
		}
	}
	return nil
}

func (s Spec) AutonomousIsolationLevel() IsolationGuarantee {
	if s.AutonomousIsolation == "" {
		return IsolationUnverified
	}
	return s.AutonomousIsolation
}

func (s Spec) ReviewIsolationLevel() IsolationGuarantee {
	if s.ReviewIsolation == "" {
		return IsolationUnverified
	}
	return s.ReviewIsolation
}

// Get returns the Spec for a provider id from the default registry.
func Get(id string) (Spec, bool) {
	return defaultRegistry.Get(id)
}

// Get returns a cloned Spec for id.
func (r *Registry) Get(id string) (Spec, bool) {
	key := strings.ToLower(strings.TrimSpace(id))
	r.mu.RLock()
	value, ok := r.entries[key]
	r.mu.RUnlock()
	if !ok {
		return Spec{}, false
	}
	return cloneSpec(value), true
}

// MustGet returns the Spec or panics.
func MustGet(id string) Spec {
	return defaultRegistry.MustGet(id)
}

// MustGet returns the Spec or panics.
func (r *Registry) MustGet(id string) Spec {
	value, ok := r.Get(id)
	if !ok {
		panic(fmt.Sprintf("unknown provider spec %q", id))
	}
	return value
}

// All returns Specs in stable registration order from the default registry.
func All() []Spec {
	return defaultRegistry.All()
}

// All returns cloned Specs in stable registration order.
func (r *Registry) All() []Spec {
	r.mu.RLock()
	result := make([]Spec, 0, len(r.order))
	for _, id := range r.order {
		if entry, ok := r.entries[id]; ok {
			result = append(result, cloneSpec(entry))
		}
	}
	r.mu.RUnlock()
	return result
}

// GroupTierFor returns the configured tier for groupKey, defaulting to primary.
func (s Spec) GroupTierFor(groupKey string) GroupTier {
	key := strings.ToLower(strings.TrimSpace(groupKey))
	for _, g := range s.ModelGroups {
		if strings.ToLower(strings.TrimSpace(g.Key)) == key {
			if g.Tier != "" {
				return g.Tier
			}
			return GroupTierPrimary
		}
	}
	return GroupTierPrimary
}

// DefaultModelGroup returns the group a provider attributes quota to when no
// more specific model carries its own pool. Codex and AGY expose a single
// primary pool, so the first primary group is the default.
func (s Spec) DefaultModelGroup() (string, bool) {
	for _, g := range s.ModelGroups {
		if s.GroupTierFor(g.Key) != GroupTierPrimary {
			continue
		}
		if key := strings.TrimSpace(g.Key); key != "" {
			return key, true
		}
	}
	return "", false
}

// ResolveGroupForModel maps a user-specified model name to a declared group key.
func (s Spec) ResolveGroupForModel(modelName string) (string, bool) {
	m := strings.ToLower(strings.TrimSpace(modelName))
	if m == "" {
		return "", false
	}
	for _, g := range s.ModelGroups {
		for _, alias := range g.Aliases {
			if a := strings.ToLower(strings.TrimSpace(alias)); a != "" && strings.Contains(m, a) {
				return g.Key, true
			}
		}
	}
	return "", false
}

// BinaryLookPath resolves provider binaries. It defaults to the
// PATH-plus-developer-dirs lookup and is a variable (not a direct call) so
// tests can stub binary presence without depending on host state.
var BinaryLookPath = runtime.LookPath

// ResolveBinary tries BinaryCandidates via BinaryLookPath.
func (s Spec) ResolveBinary() (string, error) {
	for _, candidate := range s.BinaryCandidates {
		if bin, err := BinaryLookPath(candidate); err == nil {
			return bin, nil
		}
	}
	return "", fmt.Errorf("%s binary not found (tried %s)", s.ID, strings.Join(s.BinaryCandidates, ", "))
}

// OverlayEnv applies Spec env keys and deletions onto base.
func (s Spec) OverlayEnv(base []string, profileHome, binDir, profileName string) []string {
	if s.EnvKeys == nil {
		return append([]string(nil), base...)
	}
	return overlayEnv(base, s.EnvKeys(profileHome, binDir, profileName), s.EnvDelete)
}

// NormalizeFlags maps canonical aliases for this provider.
func (s Spec) NormalizeFlags(args []string) []string {
	return Normalize(s.ID, args, nil)
}

// cloneSpec prevents callers from mutating the canonical provider registry.
func cloneSpec(s Spec) Spec {
	s.BinaryCandidates = slices.Clone(s.BinaryCandidates)
	s.EnvDelete = slices.Clone(s.EnvDelete)
	s.AutoApproval = slices.Clone(s.AutoApproval)
	s.ReviewMode = slices.Clone(s.ReviewMode)
	s.KnownVersions = slices.Clone(s.KnownVersions)
	if s.ModelGroups != nil {
		groups := make([]ModelGroupSpec, len(s.ModelGroups))
		for i, g := range s.ModelGroups {
			g.Aliases = slices.Clone(g.Aliases)
			g.RequiredWindows = slices.Clone(g.RequiredWindows)
			groups[i] = g
		}
		s.ModelGroups = groups
	}
	s.HistorySyncPolicy.SharedPaths = slices.Clone(s.HistorySyncPolicy.SharedPaths)
	s.HistorySyncPolicy.CredentialPaths = slices.Clone(s.HistorySyncPolicy.CredentialPaths)
	if s.Flags != nil {
		flags := make(FlagTable, len(s.Flags))
		for key, alias := range s.Flags {
			if alias.ProviderFlags != nil {
				m := make(map[string][]string, len(alias.ProviderFlags))
				for provider, values := range alias.ProviderFlags {
					m[provider] = slices.Clone(values)
				}
				alias.ProviderFlags = m
			}
			flags[key] = alias
		}
		s.Flags = flags
	}
	return s
}
