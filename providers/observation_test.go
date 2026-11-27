package providers

import (
	"testing"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

func TestReusableIdleObservation(t *testing.T) {
	now := time.Now()
	s := model.UsageSnapshot{FetchedAt: now, Status: model.UsageLive, Windows: []model.UsageWindow{{Kind: "5h"}}}
	if !ReusableIdleObservation(s, now, time.Minute) || ReusableIdleObservation(s, now.Add(time.Minute), time.Minute) {
		t.Fatal("wrong TTL boundary")
	}
	expiry := now
	s.ExpiresAt = &expiry
	if ReusableIdleObservation(s, now, time.Minute) {
		t.Fatal("provider expiry must win")
	}
	s.ExpiresAt = nil
	s.Status = model.UsageError
	if ReusableIdleObservation(s, now, time.Minute) {
		t.Fatal("error is not reusable capacity")
	}
}

func TestDuplicateIdentityHint(t *testing.T) {
	if DuplicateIdentityHint("cursor", "a@example.test", " ID ", true) != "id" ||
		DuplicateIdentityHint("claude", " A@example.test ", "id", true) != "a@example.test" ||
		DuplicateIdentityHint("codex", "a@example.test", "id", false) != "" {
		t.Fatal("unexpected identity hint")
	}
}
