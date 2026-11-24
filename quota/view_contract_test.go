package quota

import (
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

func TestMissingPercentIsNotMeasuredZero(t *testing.T) {
	v := BuildQuotaView(model.UsageSnapshot{Status: model.UsageLive, Windows: []model.UsageWindow{{Kind: "5h"}}}, "", "")
	if v.Available || !v.AvailReasons.UnknownQuota {
		t.Fatalf("missing measurement must not imply available quota: %+v", v)
	}
}

func TestIndependentQuotaGroups(t *testing.T) {
	zero, available := 0.0, 70.0
	v := BuildQuotaView(model.UsageSnapshot{Status: model.UsageLive, Windows: []model.UsageWindow{
		{Group: "gemini", Kind: "5h", RemainingPercent: &zero},
		{Group: "claude_gpt", Kind: "5h", RemainingPercent: &available},
	}}, "", "")
	if !v.Available || len(v.AvailReasons.ExhaustedWindows) != 1 {
		t.Fatalf("partial exhaustion must preserve independent capacity: %+v", v)
	}
	if best, ok := v.BestGroupRemaining(); !ok || best != available {
		t.Fatalf("best group remaining = %v, %v", best, ok)
	}
}
