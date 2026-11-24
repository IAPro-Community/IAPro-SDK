// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE.

package hud

import "strings"

func ChoosePrimary(state HUDState) HUDPrimary {
	for _, alert := range state.Alerts {
		if alert.Severity == "CRITICAL" {
			return HUDPrimary{Kind: HUDPrimaryAlert, ID: alert.Fingerprint}
		}
	}
	for _, mission := range state.Missions {
		if mission.NeedsUser {
			return HUDPrimary{Kind: HUDPrimaryMission, ID: mission.RunID}
		}
	}
	for _, mission := range state.Missions {
		if mission.State == "BLOCKED_NEEDS_USER" {
			return HUDPrimary{Kind: HUDPrimaryMission, ID: mission.RunID}
		}
	}
	for _, agent := range state.Agents {
		if agent.Status == "WORKING" {
			return HUDPrimary{Kind: HUDPrimaryAgent, ID: agent.ID}
		}
	}
	if state.Recommendation != nil {
		return HUDPrimary{Kind: HUDPrimaryResource, ID: strings.Trim(state.Recommendation.Provider+":"+state.Recommendation.Profile, ":")}
	}
	if len(state.Providers) > 0 {
		return HUDPrimary{Kind: HUDPrimaryCapacity}
	}
	return HUDPrimary{Kind: HUDPrimaryEmpty}
}
