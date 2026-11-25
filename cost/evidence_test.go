package cost

import (
	"math"
	"testing"
)

func TestCostEvidenceAndBudget(t *testing.T) {
	zero, paid := int64(0), int64(20)
	s, err := Summarize([]Observation{{ID: "one", MicroUSD: &zero}, {ID: "two", MicroUSD: &paid}})
	if err != nil || s.Evidence != Measured || !CanReserve(s, 25, 5) || CanReserve(s, 25, 6) {
		t.Fatalf("measured budget: %+v %v", s, err)
	}
	s, err = Summarize([]Observation{{ID: "one", MicroUSD: &paid}, {ID: "two"}})
	if err != nil || s.Evidence != Partial || CanReserve(s, 100, 1) {
		t.Fatal("unknown cost must not authorize paid fallback")
	}
	maximum := int64(math.MaxInt64)
	if _, err := Summarize([]Observation{{ID: "a", MicroUSD: &maximum}, {ID: "b", MicroUSD: &paid}}); err == nil {
		t.Fatal("cost overflow accepted")
	}
}
