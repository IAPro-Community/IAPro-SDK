// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE.

package hud

import "time"

const (
	HUDPrimaryAlert    = "ALERT"
	HUDPrimaryMission  = "MISSION"
	HUDPrimaryAgent    = "AGENT"
	HUDPrimaryResource = "RESOURCE"
	HUDPrimaryCapacity = "CAPACITY"
	HUDPrimaryEmpty    = "EMPTY"
)

type HUDWindow struct {
	Group            string     `json:"group,omitempty"`
	Kind             string     `json:"kind"`
	Label            string     `json:"label,omitempty"`
	RemainingPercent *float64   `json:"remaining_percent,omitempty"`
	ResetDescription string     `json:"reset_description,omitempty"`
	ResetAt          *time.Time `json:"reset_at,omitempty"`
	Status           string     `json:"status"`
}

type HUDProvider struct {
	Provider       string      `json:"provider"`
	Profile        string      `json:"profile"`
	DisplayName    string      `json:"display_name,omitempty"`
	Health         string      `json:"health"`
	Availability   string      `json:"availability"`
	CapacityState  string      `json:"capacity_state"`
	Source         string      `json:"source,omitempty"`
	Freshness      string      `json:"freshness"`
	LastUpdated    *time.Time  `json:"last_updated,omitempty"`
	RateLimited    bool        `json:"rate_limited"`
	LimitingWindow *HUDWindow  `json:"limiting_window,omitempty"`
	Windows        []HUDWindow `json:"windows"`
}

type HUDRecommendation struct {
	Provider     string   `json:"provider"`
	Profile      string   `json:"profile"`
	DisplayName  string   `json:"display_name,omitempty"`
	Score        float64  `json:"score"`
	Confidence   string   `json:"confidence"`
	Explanation  string   `json:"explanation"`
	Alternatives []string `json:"alternatives,omitempty"`
	Context      string   `json:"context"`
}

type HUDAgent struct {
	ID        string     `json:"id"`
	ProjectID string     `json:"project_id"`
	Name      string     `json:"name"`
	Role      string     `json:"role,omitempty"`
	Status    string     `json:"status"`
	Provider  string     `json:"provider,omitempty"`
	Profile   string     `json:"profile,omitempty"`
	Model     string     `json:"model,omitempty"`
	RuntimeID string     `json:"runtime_id,omitempty"`
	StartedAt *time.Time `json:"started_at,omitempty"`
}

type HUDMission struct {
	RunID              string    `json:"run_id"`
	ProjectID          string    `json:"project_id"`
	State              string    `json:"state"`
	CurrentStage       string    `json:"current_stage"`
	CurrentTaskID      string    `json:"current_task_id,omitempty"`
	CurrentTaskTitle   string    `json:"current_task_title,omitempty"`
	Provider           string    `json:"provider,omitempty"`
	Profile            string    `json:"profile,omitempty"`
	CompletedPackages  int       `json:"completed_packages"`
	TotalPackages      int       `json:"total_packages"`
	NeedsUser          bool      `json:"needs_user"`
	InterventionReason string    `json:"intervention_reason,omitempty"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type HUDAlert struct {
	Fingerprint      string     `json:"fingerprint"`
	Category         string     `json:"category"`
	Severity         string     `json:"severity"`
	Provider         string     `json:"provider,omitempty"`
	Profile          string     `json:"profile,omitempty"`
	Reason           string     `json:"reason,omitempty"`
	RemainingPercent *float64   `json:"remaining_percent,omitempty"`
	Acked            bool       `json:"acked"`
	SnoozedUntil     *time.Time `json:"snoozed_until,omitempty"`
	LastObservedAt   time.Time  `json:"last_observed_at"`
}

type HUDCostKindTotal struct {
	Kind     string `json:"kind"`
	MicroUSD int64  `json:"microusd"`
	Count    int    `json:"count"`
}

type HUDCostSummary struct {
	Evidence                string             `json:"evidence"`
	TokenEvidence           string             `json:"token_evidence"`
	KnownMicroUSD           int64              `json:"known_microusd"`
	UnknownObservationCount int64              `json:"unknown_observation_count"`
	BudgetLevel             string             `json:"budget_level"`
	ByKind                  []HUDCostKindTotal `json:"by_kind"`
	GeneratedAt             time.Time          `json:"generated_at,omitempty"`
}

type HUDPrimary struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}

type HUDSystemState struct {
	Capacity       string `json:"capacity"`
	Resources      string `json:"resources"`
	Agents         string `json:"agents"`
	Missions       string `json:"missions"`
	Cost           string `json:"cost"`
	CacheTTLMillis int64  `json:"cache_ttl_millis"`
}

type HUDState struct {
	GeneratedAt    time.Time          `json:"generated_at"`
	ProjectID      string             `json:"project_id,omitempty"`
	Providers      []HUDProvider      `json:"providers"`
	Recommendation *HUDRecommendation `json:"recommendation,omitempty"`
	Agents         []HUDAgent         `json:"agents"`
	Missions       []HUDMission       `json:"missions"`
	Alerts         []HUDAlert         `json:"alerts"`
	Cost           *HUDCostSummary    `json:"cost,omitempty"`
	Primary        HUDPrimary         `json:"primary"`
	System         HUDSystemState     `json:"system"`
}
