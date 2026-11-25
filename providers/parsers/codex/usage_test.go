package codex

import (
	"testing"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

func TestParseUsage(t *testing.T) {
	scope := model.AccountScope{ProviderID: "codex", ProfileID: "profile", AccountID: "local-account", IdentityVersion: "v1"}
	now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	raw := []byte(`{"accountId":"external","rateLimits":{"primary":{"usedPercent":25}}}`)
	snap, err := ParseUsage(raw, scope, "external", now)
	if err != nil || snap.AccountScope != scope || len(snap.Windows) != 1 || snap.Windows[0].RemainingPercent == nil || *snap.Windows[0].RemainingPercent != 75 {
		t.Fatalf("unexpected snapshot: %+v %v", snap, err)
	}
	if _, err := ParseUsage(raw, scope, "other", now); err == nil {
		t.Fatal("foreign account accepted")
	}
	raw = []byte(`{"accountId":"external","ordinaryUsageAllowed":false,"rateLimitsByLimitId":{"codex":{"primary":{}}}}`)
	snap, err = ParseUsage(raw, scope, "external", now)
	if err != nil || snap.Status != model.UsageRateLimited || snap.Windows[0].RemainingPercent != nil {
		t.Fatalf("backend block or missing percent ignored: %+v %v", snap, err)
	}
	raw = []byte(`{"accountId":"external","rateLimitsByLimitId":{"other":{"primary":{"usedPercent":0}}}}`)
	if _, err := ParseUsage(raw, scope, "external", now); err == nil {
		t.Fatal("unrelated quota pool accepted")
	}
}
