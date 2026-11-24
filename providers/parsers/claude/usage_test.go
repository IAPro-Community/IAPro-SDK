package claude

import "testing"

func TestParseUsage(t *testing.T) {
	w, err := ParseUsage([]byte(`{"five_hour":{"utilization":35,"resets_at":"2026-09-13T22:00:00Z"}}`))
	if err != nil || len(w) != 1 || w[0].RemainingPercent == nil || *w[0].RemainingPercent != 65 {
		t.Fatalf("unexpected quota: %+v %v", w, err)
	}
	if _, err := ParseUsage([]byte("{")); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}
