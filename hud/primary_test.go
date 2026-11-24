package hud

import "testing"

func TestChoosePrimary(t *testing.T) {
	s := HUDState{Agents: []HUDAgent{{ID: "a", Status: "WORKING"}}, Alerts: []HUDAlert{{Fingerprint: "alert", Severity: "CRITICAL"}}}
	if got := ChoosePrimary(s); got.Kind != HUDPrimaryAlert || got.ID != "alert" {
		t.Fatalf("critical alert not prioritized: %+v", got)
	}
	s.Alerts = nil
	if got := ChoosePrimary(s); got.Kind != HUDPrimaryAgent || got.ID != "a" {
		t.Fatalf("working agent not projected: %+v", got)
	}
	if got := ChoosePrimary(HUDState{}); got.Kind != HUDPrimaryEmpty {
		t.Fatalf("empty input must not invent state: %+v", got)
	}
}
