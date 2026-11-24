// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE.

package claude

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

type claudeUsagePeriod struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

type claudeUsageResponse struct {
	FiveHour       *claudeUsagePeriod `json:"five_hour"`
	SevenDay       *claudeUsagePeriod `json:"seven_day"`
	SevenDaySonnet *claudeUsagePeriod `json:"seven_day_sonnet"`
	SevenDayOpus   *claudeUsagePeriod `json:"seven_day_opus"`
}

func claudeUsageWindows(usage claudeUsageResponse) []model.UsageWindow {
	windows := make([]model.UsageWindow, 0, 4)
	appendWindow := func(period *claudeUsagePeriod, kind, group string) {
		if period == nil {
			return
		}
		used := clampPercent(period.Utilization)
		remaining := 100 - used
		window := model.UsageWindow{
			Kind:             kind,
			Group:            group,
			UsedPercent:      &used,
			RemainingPercent: &remaining,
		}
		if reset, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(period.ResetsAt)); err == nil {
			reset = reset.UTC()
			window.ResetTime = &reset
			window.ResetDescription = reset.Format(time.RFC3339)
		}
		windows = append(windows, window)
	}
	appendWindow(usage.FiveHour, "5h", "")
	appendWindow(usage.SevenDay, "weekly", "")
	appendWindow(usage.SevenDayOpus, "weekly", "opus")
	appendWindow(usage.SevenDaySonnet, "weekly", "sonnet")
	return windows
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

// ParseUsage decodes a quota response already retrieved by an authorized host.
func ParseUsage(raw []byte) ([]model.UsageWindow, error) {
	var response claudeUsageResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	return claudeUsageWindows(response), nil
}
