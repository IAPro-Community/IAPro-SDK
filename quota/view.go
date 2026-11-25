// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE for source and copyright.

package quota

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers/spec"
)

// QuotaView is the omnibus view model for quota display and scoring.
// Consumed by TUI, Web API, CLI, and Scheduler.
type QuotaView struct {
	Scope     model.AccountScope `json:"account_scope"`
	ExpiresAt *time.Time         `json:"expires_at,omitempty"`

	Provider          string       `json:"provider"`
	Profile           string       `json:"profile"`
	Account           string       `json:"account"`
	Plan              string       `json:"plan"`
	Status            string       `json:"status"`
	Source            string       `json:"source"`
	ModelGroups       []ModelGroup `json:"model_groups"`
	FetchedAt         time.Time    `json:"fetched_at"`
	Available         bool         `json:"available"`
	AvailReasons      AvailReasons `json:"avail_reasons"`
	DiagnosticCode    string       `json:"diagnostic_code,omitempty"`
	DiagnosticMessage string       `json:"diagnostic_message,omitempty"`
}

// AvailReasons holds structured reasons for availability status.
type AvailReasons struct {
	ExhaustedWindows []string `json:"exhausted_windows,omitempty"`
	RateLimited      bool     `json:"rate_limited,omitempty"`
	UnknownQuota     bool     `json:"unknown_quota,omitempty"`
	AuthRequired     bool     `json:"auth_required,omitempty"`
	AllOK            bool     `json:"all_ok,omitempty"`
}

// ModelGroup clusters usage windows by model family.
// A provider like AGY exposes two groups (Gemini as primary, Claude/GPT as auxiliary).
// Simpler providers have a single primary group.
type ModelGroup struct {
	Key     string         `json:"key,omitempty"` // stable identity: gemini, claude_gpt, …
	Name    string         `json:"name"`
	Tier    spec.GroupTier `json:"tier,omitempty"`
	Windows []Window       `json:"windows"`
}

// Window is a display-ready usage window within a model group.
type Window struct {
	Kind      string     `json:"kind"`
	Label     string     `json:"label"`
	Remaining float64    `json:"remaining"`
	ResetDesc string     `json:"reset_desc"`
	ResetTime *time.Time `json:"reset_time,omitempty"`
	Status    string     `json:"status"`
	Bar       string     `json:"bar"`
}

// Bottleneck returns the minimum remaining percentage across ALL windows and groups.
// The second return value identifies the bottleneck window kind.
// This is the tightest window for warnings ("Gemini 5h 0%"), not the account score.
// Prefer BestGroupRemaining for scheduling / quota_remaining.
func (qv *QuotaView) Bottleneck() (float64, string) {
	minPct := 100.0
	var bottleneckKind string
	found := false
	now := time.Now()
	for _, g := range qv.ModelGroups {
		for _, w := range g.Windows {
			if w.Kind == "unknown" || windowExpired(w, now) {
				continue
			}
			found = true
			if bottleneckKind == "" || w.Remaining < minPct {
				minPct = w.Remaining
				bottleneckKind = w.Kind
			}
		}
	}
	if !found {
		return 0, ""
	}
	return minPct, bottleneckKind
}

// GroupRemaining returns the capacity of one model group: min of its known
// windows (5h and weekly stack in the same pool). ok is false when the group
// has no scorable windows. Windows whose reset time is already past are ignored
// (the quota window has rolled over; stale used% must not look like exhaustion).
func (g ModelGroup) GroupRemaining() (remaining float64, ok bool) {
	return g.GroupRemainingAt(time.Now())
}

// GroupRemainingAt returns the capacity of one model group at the caller's clock.
func (g ModelGroup) GroupRemainingAt(now time.Time) (remaining float64, ok bool) {
	minPct := 100.0
	found := false
	for _, w := range g.Windows {
		if w.Kind == "unknown" || windowExpired(w, now) {
			continue
		}
		found = true
		if w.Remaining < minPct {
			minPct = w.Remaining
		}
	}
	if !found {
		return 0, false
	}
	return minPct, true
}

// EffectiveTier returns the group's tier, defaulting to GroupTierPrimary.
func (g ModelGroup) EffectiveTier() spec.GroupTier {
	if g.Tier == spec.GroupTierAuxiliary {
		return spec.GroupTierAuxiliary
	}
	return spec.GroupTierPrimary
}

func windowExpired(w Window, now time.Time) bool {
	return w.ResetTime != nil && !now.Before(*w.ResetTime)
}

// BestGroupRemaining returns the best usable pool capacity: max of each
// group's GroupRemaining. Independent families (AGY Gemini vs Claude) do not
// cancel each other; a single-pool provider (Codex) collapses to min(5h, weekly).
func (qv *QuotaView) BestGroupRemaining() (remaining float64, ok bool) {
	return qv.BestGroupRemainingAt(time.Now())
}

// BestGroupRemainingAt returns the max remaining capacity across all groups at now.
func (qv *QuotaView) BestGroupRemainingAt(now time.Time) (remaining float64, ok bool) {
	best := 0.0
	found := false
	for _, g := range qv.ModelGroups {
		rem, groupOK := g.GroupRemainingAt(now)
		if !groupOK {
			continue
		}
		if !found || rem > best {
			best = rem
			found = true
		}
	}
	return best, found
}

// PrimaryGroupRemaining returns the best remaining percentage among primary model groups.
func (qv *QuotaView) PrimaryGroupRemaining() (remaining float64, ok bool) {
	return qv.PrimaryGroupRemainingAt(time.Now())
}

// PrimaryGroupRemainingAt returns the best remaining percentage among primary groups at now.
func (qv *QuotaView) PrimaryGroupRemainingAt(now time.Time) (remaining float64, ok bool) {
	return qv.tierRemainingAt(spec.GroupTierPrimary, now)
}

// AuxiliaryGroupRemaining returns the best remaining percentage among auxiliary model groups.
func (qv *QuotaView) AuxiliaryGroupRemaining() (remaining float64, ok bool) {
	return qv.AuxiliaryGroupRemainingAt(time.Now())
}

// AuxiliaryGroupRemainingAt returns the best remaining percentage among auxiliary groups at now.
func (qv *QuotaView) AuxiliaryGroupRemainingAt(now time.Time) (remaining float64, ok bool) {
	return qv.tierRemainingAt(spec.GroupTierAuxiliary, now)
}

func (qv *QuotaView) tierRemainingAt(tier spec.GroupTier, now time.Time) (remaining float64, ok bool) {
	if qv == nil {
		return 0, false
	}
	best := 0.0
	found := false
	for _, g := range qv.ModelGroups {
		if g.EffectiveTier() != tier {
			continue
		}
		rem, groupOK := g.GroupRemainingAt(now)
		if !groupOK {
			continue
		}
		if !found || rem > best {
			best = rem
			found = true
		}
	}
	return best, found
}

// GroupRemainingForKeyAt returns the remaining capacity for a specific group key.
func (qv *QuotaView) GroupRemainingForKeyAt(groupKey string, now time.Time) (remaining float64, ok bool) {
	if qv == nil {
		return 0, false
	}
	target := strings.ToLower(strings.TrimSpace(groupKey))
	for _, g := range qv.ModelGroups {
		if strings.ToLower(strings.TrimSpace(g.Key)) == target {
			return g.GroupRemainingAt(now)
		}
	}
	return 0, false
}

// NeedsResetReprobeAt reports whether any previously recorded window has reached
// its ResetTime by now, indicating cached exhaustion should be refreshed.
func (qv *QuotaView) NeedsResetReprobeAt(now time.Time) bool {
	if qv == nil {
		return false
	}
	for _, g := range qv.ModelGroups {
		for _, w := range g.Windows {
			if w.ResetTime != nil && !now.Before(*w.ResetTime) {
				return true
			}
		}
	}
	return false
}

// UsableGroupCount returns how many model groups still have capacity (> 0).
func (qv *QuotaView) UsableGroupCount() int {
	n := 0
	for _, g := range qv.ModelGroups {
		rem, ok := g.GroupRemaining()
		if ok && rem > 0 {
			n++
		}
	}
	return n
}

// TotalGroupRemaining sums each group's GroupRemaining (zeros included when
// the group has scorable windows). Used with UsableGroupCount for scheduling.
func (qv *QuotaView) TotalGroupRemaining() (total float64, groups int) {
	for _, g := range qv.ModelGroups {
		rem, ok := g.GroupRemaining()
		if !ok {
			continue
		}
		total += rem
		groups++
	}
	return total, groups
}

// EffectiveCapacityScore encodes "most available" for tier-aware scheduling:
// primary quotas (>0%) occupy [1000..2001] and always beat auxiliary-only quotas
// (0..100]. Within the primary tier, higher primary remaining always wins;
// auxiliary capacity serves only as a tiny tie-breaker.
func (qv *QuotaView) EffectiveCapacityScore() (score float64, ok bool) {
	return qv.EffectiveCapacityScoreAt(time.Now())
}

// EffectiveCapacityScoreAt computes the tier-aware scheduling score at now.
func (qv *QuotaView) EffectiveCapacityScoreAt(now time.Time) (score float64, ok bool) {
	primRem, primOK := qv.PrimaryGroupRemainingAt(now)
	auxRem, auxOK := qv.AuxiliaryGroupRemainingAt(now)
	if !primOK && !auxOK {
		return 0, false
	}
	if primOK && primRem > 0 {
		tie := 0.0
		if auxOK && auxRem > 0 {
			tie = auxRem * 0.001
		}
		return 1000.0 + primRem*10.0 + tie, true
	}
	if auxOK && auxRem > 0 {
		return auxRem, true
	}
	return 0, true
}

// CompareCapacityAt compares two QuotaViews with tier-first ordering.
// Returns > 0 if qv has strictly better available capacity than other.
func (qv *QuotaView) CompareCapacityAt(other *QuotaView, now time.Time) int {
	s1, ok1 := qv.EffectiveCapacityScoreAt(now)
	s2, ok2 := other.EffectiveCapacityScoreAt(now)
	switch {
	case ok1 && !ok2:
		return 1
	case !ok1 && ok2:
		return -1
	case !ok1 && !ok2:
		return 0
	case s1 > s2:
		return 1
	case s1 < s2:
		return -1
	default:
		return 0
	}
}

// EffectiveCapacityRatio normalizes EffectiveCapacityScore to 0-1 for APIs
// that expect quota_remaining in that range (primary remaining when available,
// otherwise auxiliary remaining).
func (qv *QuotaView) EffectiveCapacityRatio() (ratio float64, ok bool) {
	return qv.EffectiveCapacityRatioAt(time.Now())
}

// EffectiveCapacityRatioAt returns the normalized 0..1 capacity ratio at now.
func (qv *QuotaView) EffectiveCapacityRatioAt(now time.Time) (ratio float64, ok bool) {
	primRem, primOK := qv.PrimaryGroupRemainingAt(now)
	if primOK && primRem > 0 {
		return clampUnit(primRem / 100.0), true
	}
	auxRem, auxOK := qv.AuxiliaryGroupRemainingAt(now)
	if auxOK && auxRem > 0 {
		return clampUnit(auxRem / 100.0), true
	}
	if primOK || auxOK {
		return 0, true
	}
	return 0, false
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// QuotaTruthState is the canonical cross-surface state consumed by CLI, TUI, and Web.
type QuotaTruthState string

const (
	TruthConfirmed       QuotaTruthState = "confirmed"
	TruthStale           QuotaTruthState = "stale"
	TruthExhausted       QuotaTruthState = "exhausted"
	TruthRateLimited     QuotaTruthState = "rate_limited"
	TruthUnknown         QuotaTruthState = "unknown"
	TruthUnauthenticated QuotaTruthState = "unauthenticated"
	TruthUnsupported     QuotaTruthState = "unsupported"
)

// QuotaProjection is the canonical read model shared across CLI, TUI, and Web.
type QuotaProjection struct {
	TruthState         QuotaTruthState `json:"truth_state"`
	Available          bool            `json:"available"`
	PrimaryAvailable   bool            `json:"primary_available"`
	AuxiliaryOnly      bool            `json:"auxiliary_only"`
	ActiveTier         spec.GroupTier  `json:"active_tier,omitempty"`
	PrimaryRemaining   *float64        `json:"primary_remaining,omitempty"`
	AuxiliaryRemaining *float64        `json:"auxiliary_remaining,omitempty"`
	EffectiveRemaining *float64        `json:"effective_remaining,omitempty"`
	BottleneckKind     string          `json:"bottleneck_kind,omitempty"`
	CompactSummary     string          `json:"compact_summary,omitempty"`
	DiagnosticCode     string          `json:"diagnostic_code,omitempty"`
	DiagnosticMessage  string          `json:"diagnostic_message,omitempty"`
	FreshnessSeconds   int64           `json:"freshness_seconds,omitempty"`
	NeedsResetReprobe  bool            `json:"needs_reset_reprobe,omitempty"`
	RankScore          float64         `json:"rank_score"`
}

// ProjectionAt builds the single canonical QuotaProjection for any UI or router.
func (qv *QuotaView) ProjectionAt(now time.Time, ttl time.Duration, authenticated bool, health string) QuotaProjection {
	out := QuotaProjection{
		TruthState: TruthUnknown,
	}
	if qv == nil {
		if !authenticated {
			out.TruthState = TruthUnauthenticated
			out.RankScore = -500
		} else {
			out.RankScore = -400
		}
		return out
	}

	view := *qv
	view.ComputeAvailabilityAt(now)
	out.DiagnosticCode = view.DiagnosticCode
	out.DiagnosticMessage = view.DiagnosticMessage
	out.CompactSummary = view.CompactGroupSummary()
	_, out.BottleneckKind = view.Bottleneck()
	out.NeedsResetReprobe = view.NeedsResetReprobeAt(now)
	if !view.FetchedAt.IsZero() {
		age := int64(now.Sub(view.FetchedAt).Seconds())
		if age < 0 {
			age = 0
		}
		out.FreshnessSeconds = age
	}

	primRem, primOK := view.PrimaryGroupRemainingAt(now)
	if primOK {
		v := primRem
		out.PrimaryRemaining = &v
		if v > 0 {
			out.PrimaryAvailable = true
		}
	}
	auxRem, auxOK := view.AuxiliaryGroupRemainingAt(now)
	if auxOK {
		v := auxRem
		out.AuxiliaryRemaining = &v
	}

	if out.PrimaryAvailable {
		v := primRem
		out.EffectiveRemaining = &v
		out.ActiveTier = spec.GroupTierPrimary
	} else if auxOK && auxRem > 0 {
		v := auxRem
		out.EffectiveRemaining = &v
		out.AuxiliaryOnly = true
		out.ActiveTier = spec.GroupTierAuxiliary
	} else if primOK {
		zero := 0.0
		out.EffectiveRemaining = &zero
	}

	normHealth := strings.ToLower(strings.TrimSpace(health))
	status := strings.ToUpper(strings.TrimSpace(view.Status))

	if !authenticated || normHealth == strings.ToLower(string(model.HealthAuthRequired)) || normHealth == "auth_required" || view.AvailReasons.AuthRequired {
		out.TruthState = TruthUnauthenticated
		out.Available = false
		out.RankScore = -500
		return out
	}
	if status == string(model.UsageRateLimited) || view.AvailReasons.RateLimited || normHealth == strings.ToLower(string(model.HealthRateLimited)) {
		out.TruthState = TruthRateLimited
		out.Available = false
		out.RankScore = -350
		return out
	}
	if status == string(model.UsageUnsupported) {
		out.TruthState = TruthUnsupported
		out.Available = view.Available
		out.RankScore = -200
		return out
	}
	if status == string(model.UsageUnknown) || status == string(model.UsageError) || view.AvailReasons.UnknownQuota || (!primOK && !auxOK) {
		out.TruthState = TruthUnknown
		out.Available = false
		out.RankScore = -400
		return out
	}
	if !view.Available || (!out.PrimaryAvailable && (!auxOK || auxRem <= 0)) {
		out.TruthState = TruthExhausted
		out.Available = false
		out.RankScore = -300
		return out
	}

	out.Available = true
	stale := status == string(model.UsageEstimated) || out.NeedsResetReprobe
	if !stale && ttl > 0 {
		if view.FetchedAt.IsZero() || now.Sub(view.FetchedAt) > ttl {
			stale = true
		}
	}
	if status == string(model.UsageLive) && !stale {
		out.TruthState = TruthConfirmed
	} else if status == string(model.UsageCached) && !stale {
		out.TruthState = TruthConfirmed
	} else {
		out.TruthState = TruthStale
	}

	if score, ok := view.EffectiveCapacityScoreAt(now); ok {
		out.RankScore = score
	}
	return out
}

// CompactGroupSummary lists each group's capacity for compact UIs
// ("Gemini 0% · Claude 100%" or "5h 70% · weekly 95%" for a single pool).
func (qv *QuotaView) CompactGroupSummary() string {
	if qv == nil || len(qv.ModelGroups) == 0 {
		return ""
	}
	if qv.HasMultipleGroups() {
		parts := make([]string, 0, len(qv.ModelGroups))
		for _, g := range qv.ModelGroups {
			rem, ok := g.GroupRemaining()
			if !ok {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s %.0f%%", shortGroupLabel(g), rem))
		}
		return strings.Join(parts, " · ")
	}
	g := qv.ModelGroups[0]
	parts := make([]string, 0, len(g.Windows))
	for _, w := range g.Windows {
		if w.Kind == "unknown" || math.IsNaN(w.Remaining) || math.IsInf(w.Remaining, 0) || w.Remaining < 0 || w.Remaining > 100 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %.0f%%", shortWindowLabel(w.Kind), w.Remaining))
	}
	return strings.Join(parts, " · ")
}

func shortGroupLabel(g ModelGroup) string {
	key := g.Key
	if key == "" {
		key = strings.ToLower(g.Name)
	}
	switch {
	case key == "gemini" || strings.Contains(key, "gemini"):
		return "Gemini"
	case key == "claude_gpt" || strings.Contains(key, "claude") || strings.Contains(key, "gpt"):
		return "Claude"
	case strings.TrimSpace(g.Name) != "":
		return g.Name
	default:
		return "Quota"
	}
}

func shortWindowLabel(kind string) string {
	switch kind {
	case "5h", "daily", "claude_5h", "claude_five_hour":
		return "5h"
	case "weekly", "claude_weekly":
		return "weekly"
	default:
		return kind
	}
}

// AllWindows returns a flat list of all windows across all groups.
// Used by scheduler for multi-window bottleneck scoring.
func (qv *QuotaView) AllWindows() []Window {
	var all []Window
	for _, g := range qv.ModelGroups {
		all = append(all, g.Windows...)
	}
	return all
}

// HasMultipleGroups returns true when the provider exposes separate model families.
func (qv *QuotaView) HasMultipleGroups() bool {
	return len(qv.ModelGroups) > 1
}

// IsAvailable returns true if at least one model group can accept work RIGHT NOW.
// A caller selecting a specific model must still verify that model's group.
// An agent is unavailable when every known quota pool is exhausted or when
// rate-limited. This matters for providers such as AGY, which exposes separate
// Gemini and Claude/GPT capacity pools.
// UNKNOWN quota is a hard gate for automatic allocation / Mission routing —
// missing evidence is never treated as capacity.
func (qv *QuotaView) IsAvailable() bool {
	return qv.Available
}

// AvailabilityLabel returns a human-readable availability status.
func (qv *QuotaView) AvailabilityLabel() string {
	if qv.AvailReasons.UnknownQuota {
		return "SEM DADOS"
	}
	if qv.Available {
		return "DISPONIVEL"
	}
	if qv.AvailReasons.RateLimited {
		return "RATE LIMITED"
	}
	if len(qv.AvailReasons.ExhaustedWindows) > 0 {
		return "QUOTA ESGOTADA"
	}
	if qv.AvailReasons.AuthRequired {
		return "SEM AUTENTICACAO"
	}
	return "INDISPONIVEL"
}

// ComputeAvailability analyzes all windows and sets Available + AvailReasons.
func (qv *QuotaView) ComputeAvailability() {
	qv.ComputeAvailabilityAt(time.Now())
}

// ComputeAvailabilityAt recomputes availability with the caller's clock.
func (qv *QuotaView) ComputeAvailabilityAt(now time.Time) {
	var exhausted []string
	qv.Available = true
	qv.AvailReasons = AvailReasons{}

	// Check rate-limited status — hard filter.
	if qv.Status == string(model.UsageRateLimited) {
		qv.Available = false
		qv.AvailReasons.RateLimited = true
		return
	}

	// Check auth required — hard filter.
	if qv.Status == "auth_required" {
		qv.Available = false
		qv.AvailReasons.AuthRequired = true
		return
	}

	// Check UNKNOWN quota — hard gate for autopilot allocation.
	if qv.Status == string(model.UsageUnknown) || qv.Status == string(model.UsageError) {
		qv.Available = false
		qv.AvailReasons.UnknownQuota = true
		return
	}

	// Model groups are independent capacity pools. One usable group keeps the
	// profile eligible; an AGY account with Gemini quota remains usable for a
	// Gemini request even when its Claude/GPT pool is exhausted. The exhausted
	// group is still exposed in AvailReasons for honest UI feedback.
	usableGroup := false
	for _, g := range qv.ModelGroups {
		groupExhausted := false
		hasScorable := false
		for _, w := range g.Windows {
			if w.Kind == "unknown" {
				continue
			}
			if windowExpired(w, now) {
				continue
			}
			hasScorable = true
			if w.Remaining <= 0.0 {
				exhausted = append(exhausted, w.Kind)
				groupExhausted = true
			}
		}
		if hasScorable && !groupExhausted {
			usableGroup = true
		}
	}

	if !usableGroup && len(exhausted) > 0 {
		qv.Available = false
		qv.AvailReasons.ExhaustedWindows = exhausted
		return
	}
	if !usableGroup {
		qv.Available = false
		qv.AvailReasons.UnknownQuota = true
		return
	}

	// Keep partial exhaustion visible even when another model group is usable.
	qv.AvailReasons.ExhaustedWindows = exhausted
	qv.AvailReasons.AllOK = len(exhausted) == 0
}

// RenderBar renders a progress bar for a given remaining percentage.
func RenderBar(remaining float64, status string, width int) string {
	var pctPtr *float64
	if statusHasMeasuredPercent(status) {
		pctPtr = &remaining
	}
	return RenderProgressBar(model.UsageStatus(status), pctPtr, width)
}

// RenderBarWithPercent renders a progress bar with a percentage label.
func RenderBarWithPercent(remaining float64, status string, width int) string {
	bar := RenderBar(remaining, status, width)
	if statusHasMeasuredPercent(status) {
		return fmt.Sprintf("%s %2.0f%%", bar, remaining)
	}
	return bar
}

// statusHasMeasuredPercent reports whether a status is backed by a real
// measurement. RATE_LIMITED qualifies: the official provider read supplies exact
// percentages alongside the block.
func statusHasMeasuredPercent(status string) bool {
	switch model.UsageStatus(status) {
	case model.UsageLive, model.UsageCached, model.UsageEstimated, model.UsageRateLimited:
		return true
	default:
		return false
	}
}

// WindowLabel returns the display label for a window kind.
func WindowLabel(kind string) string {
	switch kind {
	case "5h", "daily":
		return "Limite 5 Horas"
	case "weekly":
		return "Limite Semanal"
	case "claude_5h", "claude_five_hour":
		return "Limite 5 Horas"
	case "claude_weekly":
		return "Limite Semanal"
	default:
		return kind
	}
}

// GroupDisplayName returns the display name for a model group.
func GroupDisplayName(groupKey string) string {
	switch groupKey {
	case "gemini":
		return "Gemini Models"
	case "claude_gpt":
		return "Claude & GPT Models"
	case "":
		return ""
	default:
		return groupKey
	}
}

// sortGroupOrder defines a stable display order for model groups.
var sortGroupOrder = map[string]int{
	"":           0,
	"gemini":     1,
	"claude_gpt": 2,
}

// SortModelGroups orders model groups for consistent display.
func SortModelGroups(groups []ModelGroup) {
	sort.Slice(groups, func(i, j int) bool {
		oi, okI := sortGroupOrder[groupSortKey(groups[i])]
		oj, okJ := sortGroupOrder[groupSortKey(groups[j])]
		if okI && okJ {
			return oi < oj
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}
		return groups[i].Name < groups[j].Name
	})
}

func groupSortKey(g ModelGroup) string {
	if g.Key != "" {
		return g.Key
	}
	name := strings.ToLower(g.Name)
	switch {
	case strings.Contains(name, "gemini"):
		return "gemini"
	case strings.Contains(name, "claude") || strings.Contains(name, "gpt"):
		return "claude_gpt"
	default:
		return g.Name
	}
}

// GroupKeys returns a sorted slice of unique group keys from windows.
func GroupKeys(windows []model.UsageWindow) []string {
	seen := make(map[string]bool)
	for _, w := range windows {
		seen[w.Group] = true
	}
	var keys []string
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		oi, okI := sortGroupOrder[keys[i]]
		oj, okJ := sortGroupOrder[keys[j]]
		if okI && okJ {
			return oi < oj
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}
		return keys[i] < keys[j]
	})
	return keys
}

// NormalizeGroupLabel removes trailing whitespace and normalizes group names for display.
func NormalizeGroupLabel(s string) string {
	return strings.TrimSpace(s)
}
