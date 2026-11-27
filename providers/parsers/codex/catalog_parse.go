package codex

import (
	"encoding/json"
	"errors"
	"sort"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

// maxCatalogBytes bounds the decoded `codex debug models` payload.
const maxCatalogBytes = 1 << 20

// Model is one entry of the Codex model catalog. Only fields the host needs to
// offer a choice are kept: slugs, labels, default reasoning effort and whether
// the CLI lists the model.
type Model struct {
	Slug             string   `json:"slug"`
	DisplayName      string   `json:"display_name"`
	Description      string   `json:"description,omitempty"`
	DefaultReasoning string   `json:"default_reasoning_level,omitempty"`
	ReasoningFactors []string `json:"reasoning_levels,omitempty"`
	Visibility       string   `json:"visibility,omitempty"`
	SupportedInAPI   bool     `json:"supported_in_api"`
	Priority         int      `json:"priority,omitempty"`
	SpeedTiers       []string `json:"additional_speed_tiers,omitempty"`
}

// Listed reports whether the CLI presents the model for normal selection.
func (m Model) Listed() bool { return m.Visibility == "" || m.Visibility == "list" }

type catalogEnvelope struct {
	Models []struct {
		Slug                  string `json:"slug"`
		DisplayName           string `json:"display_name"`
		Description           string `json:"description"`
		DefaultReasoningLevel string `json:"default_reasoning_level"`
		SupportedReasoning    []struct {
			Effort string `json:"effort"`
		} `json:"supported_reasoning_levels"`
		Visibility         string   `json:"visibility"`
		SupportedInAPI     bool     `json:"supported_in_api"`
		Priority           int      `json:"priority"`
		AdditionalSpeedTie []string `json:"additional_speed_tiers"`
	} `json:"models"`
}

// ParseCatalog decodes `codex debug models` output into a sorted catalog.
// Duplicate slugs, missing slugs and empty payloads are rejected, not guessed.
func ParseCatalog(raw []byte) ([]Model, error) {
	if len(raw) == 0 || len(raw) > maxCatalogBytes {
		return nil, errors.New("codex model catalog size out of range")
	}
	var env catalogEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, errors.New("codex model catalog is not valid JSON")
	}
	if len(env.Models) == 0 {
		return nil, errors.New("codex model catalog is empty")
	}
	seen := make(map[string]struct{}, len(env.Models))
	out := make([]Model, 0, len(env.Models))
	for _, m := range env.Models {
		if m.Slug == "" {
			return nil, errors.New("codex model without slug")
		}
		if _, dup := seen[m.Slug]; dup {
			return nil, errors.New("codex model catalog repeats a slug")
		}
		seen[m.Slug] = struct{}{}
		item := Model{
			Slug:             m.Slug,
			DisplayName:      m.DisplayName,
			Description:      m.Description,
			DefaultReasoning: m.DefaultReasoningLevel,
			Visibility:       m.Visibility,
			SupportedInAPI:   m.SupportedInAPI,
			Priority:         m.Priority,
			SpeedTiers:       m.AdditionalSpeedTie,
		}
		for _, level := range m.SupportedReasoning {
			if level.Effort != "" {
				item.ReasoningFactors = append(item.ReasoningFactors, level.Effort)
			}
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		pi, pj := out[i].Priority, out[j].Priority
		if pi == 0 || pj == 0 {
			// Unprioritized entries sort last.
			return pi != 0 && pj == 0
		}
		if pi != pj {
			return pi < pj
		}
		return out[i].Slug < out[j].Slug
	})
	return out, nil
}

// CatalogFailure reports a model that the catalog does not contain. The
// requested model name is never echoed, so no provider text leaks.
func CatalogFailure() model.Failure {
	return model.Failure{
		Kind:    model.FailureModelUnsupported,
		Message: "Requested model is not listed by the Codex CLI catalog",
	}
}
