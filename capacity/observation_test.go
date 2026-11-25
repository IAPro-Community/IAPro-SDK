package capacitymonitor

import (
	"testing"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

func TestObserveFreshAndUnknown(t *testing.T) {
	now := time.Now()
	remaining := 40.0
	scope := model.AccountScope{ProviderID: "codex", ProfileID: "a", AccountID: "a", IdentityVersion: "v1"}
	snap := model.UsageSnapshot{ProviderID: "codex", ProfileID: "a", AccountScope: scope, FetchedAt: now, Status: model.UsageLive,
		Windows: []model.UsageWindow{{Kind: "5h", RemainingPercent: &remaining}}}
	out, err := Observe("owner", snap, nil, DefaultThresholds(), now, time.Minute)
	if err != nil || len(out) != 1 || out[0].State != StateHealthy {
		t.Fatalf("fresh: %+v %v", out, err)
	}
	out, err = Observe("owner", snap, nil, DefaultThresholds(), now.Add(time.Minute), time.Minute)
	if err != nil || out[0].State != StateUnknown || out[0].Remaining != nil {
		t.Fatalf("expired: %+v %v", out, err)
	}
	snap.Status = model.UsageError
	out, err = Observe("owner", snap, nil, DefaultThresholds(), now, time.Minute)
	if err != nil || out[0].State != StateUnknown {
		t.Fatal("error status became capacity")
	}
}
