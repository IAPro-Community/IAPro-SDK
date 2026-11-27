package codex

import (
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

func TestParseCatalogSortsByPriorityAndKeepsLevels(t *testing.T) {
	models, err := ParseCatalog([]byte(`{"models":[
		{"slug":"gpt-b","display_name":"B","priority":2,"visibility":"list","supported_in_api":true,"default_reasoning_level":"low","supported_reasoning_levels":[{"effort":"low"},{"effort":"high"}]},
		{"slug":"gpt-a","display_name":"A","priority":1,"visibility":"list","supported_in_api":true}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].Slug != "gpt-a" || models[1].DefaultReasoning != "low" {
		t.Fatalf("unexpected catalog: %+v", models)
	}
	if len(models[1].ReasoningFactors) != 2 || models[1].ReasoningFactors[1] != "high" {
		t.Fatalf("reasoning levels lost: %+v", models[1])
	}
	if !models[0].Listed() {
		t.Fatal("list entries must be listed")
	}
}

func TestParseCatalogRejectsBadPayloads(t *testing.T) {
	cases := map[string]string{
		"empty":    `{}`,
		"invalid":  `not json`,
		"no slug":  `{"models":[{"display_name":"x"}]}`,
		"dup slug": `{"models":[{"slug":"a"},{"slug":"a"}]}`,
		"zero":     ``,
	}
	for name, raw := range cases {
		if _, err := ParseCatalog([]byte(raw)); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
}

func TestParseCatalogHidesUnlistedModels(t *testing.T) {
	models, err := ParseCatalog([]byte(`{"models":[{"slug":"hidden","visibility":"hide","supported_in_api":true},{"slug":"shown","visibility":"list","supported_in_api":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if models[0].Slug != "hidden" || !models[1].Listed() || models[0].Listed() {
		t.Fatalf("unexpected order or listing: %+v", models)
	}
}

func TestCatalogFailureIsUnsupportedModel(t *testing.T) {
	f := CatalogFailure()
	if f.Kind != model.FailureModelUnsupported || f.Message == "" {
		t.Fatalf("unexpected failure: %+v", f)
	}
}
