package gemini

import "testing"

func TestParseUsage(t *testing.T) {
	w, err := ParseUsage([]byte(`{"buckets":[{"modelId":"gemini-2.5-pro","remainingFraction":0.42,"tokenType":"REQUESTS"},{"modelId":"gemini-2.5-flash","remainingAmount":"7"}]}`))
	if err != nil || len(w) != 2 || w[0].RemainingPercent == nil || *w[0].RemainingPercent != 42 || w[1].RemainingPercent != nil {
		t.Fatalf("unexpected quota: %+v %v", w, err)
	}
}
