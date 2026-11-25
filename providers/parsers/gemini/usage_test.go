package gemini

import "testing"

func TestParseUsage(t *testing.T) {
	w, err := ParseUsage([]byte(`{"buckets":[{"modelId":"gemini-2.5-pro","remainingFraction":0.42,"tokenType":"REQUESTS"},{"modelId":"gemini-2.5-flash","remainingAmount":"7"}]}`))
	if err != nil || len(w) != 2 || w[0].RemainingPercent == nil || *w[0].RemainingPercent != 42 || w[1].RemainingPercent != nil {
		t.Fatalf("unexpected quota: %+v %v", w, err)
	}
}

func TestInvalidMeasurementsAreUnknown(t *testing.T) {
	for _, raw := range []string{
		`{"buckets":[{"modelId":"m","remainingFraction":-1,"remainingAmount":"NaN"}]}`,
		`{"buckets":[{"modelId":"m","remainingFraction":1.1,"remainingAmount":"Infinity"}]}`,
		`{"buckets":[{"modelId":"m","remainingAmount":-1}]}`,
	} {
		w, err := ParseUsage([]byte(raw))
		if err != nil || len(w) != 1 || w[0].RemainingPercent != nil || w[0].Remaining != nil {
			t.Fatalf("invalid measurement: %s: %+v %v", raw, w, err)
		}
	}
}
