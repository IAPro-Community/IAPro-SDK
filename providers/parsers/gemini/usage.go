// SPDX-License-Identifier: MIT
// Derived from IAPro Nexus; see NOTICE.

package gemini

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

type geminiQuotaResponse struct {
	Buckets []geminiQuotaBucket `json:"buckets"`
}

type geminiQuotaBucket struct {
	ModelID           string          `json:"modelId"`
	RemainingFraction *float64        `json:"remainingFraction"`
	RemainingAmount   json.RawMessage `json:"remainingAmount"`
	ResetTime         string          `json:"resetTime"`
	TokenType         string          `json:"tokenType"`
}

func parseGeminiQuotaBuckets(buckets []geminiQuotaBucket) []model.UsageWindow {
	windows := make([]model.UsageWindow, 0, len(buckets))
	for _, bucket := range buckets {
		if strings.TrimSpace(bucket.ModelID) == "" {
			continue
		}
		window := model.UsageWindow{
			Kind:  geminiQuotaKind(bucket.TokenType),
			Group: strings.TrimSpace(bucket.ModelID),
			Unit:  strings.ToLower(strings.TrimSpace(bucket.TokenType)),
		}
		if window.Unit == "" {
			window.Unit = "quota"
		}
		if bucket.RemainingFraction != nil && *bucket.RemainingFraction >= 0 && *bucket.RemainingFraction <= 1 {
			remainingPercent := *bucket.RemainingFraction * 100
			usedPercent := 100 - remainingPercent
			window.RemainingPercent = &remainingPercent
			window.UsedPercent = &usedPercent
		}
		if amount, ok := parseRemainingAmount(bucket.RemainingAmount); ok {
			window.Remaining = &amount
		}
		if reset, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(bucket.ResetTime)); err == nil {
			window.ResetTime = &reset
		}
		windows = append(windows, window)
	}
	return windows
}

func geminiQuotaKind(tokenType string) string {
	normalized := strings.ToLower(strings.TrimSpace(tokenType))
	switch normalized {
	case "request", "requests":
		return "requests"
	case "":
		return "quota"
	default:
		return normalized
	}
}

func parseRemainingAmount(raw json.RawMessage) (float64, bool) {
	value := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if value == "" || value == "null" {
		return 0, false
	}
	amount, err := strconv.ParseFloat(value, 64)
	return amount, err == nil && !math.IsNaN(amount) && !math.IsInf(amount, 0) && amount >= 0
}


// ParseUsage decodes a quota response already retrieved by an authorized host.
func ParseUsage(raw []byte) ([]model.UsageWindow, error) {
	var response geminiQuotaResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	return parseGeminiQuotaBuckets(response.Buckets), nil
}
