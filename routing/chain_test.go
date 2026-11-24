package routing

import "testing"

func TestCanAdvanceChain(t *testing.T) {
	base := ChainBoundary{Authorized: true, Retryable: true}
	if !CanAdvanceChain(base) {
		t.Fatal("explicit text-only chain should advance")
	}
	for _, b := range []ChainBoundary{
		{Retryable: true},
		{Authorized: true},
		{Authorized: true, Retryable: true, Canceled: true},
		{Authorized: true, Retryable: true, BudgetOrLedger: true},
		{Authorized: true, Retryable: true, OutputEmitted: true},
		{Authorized: true, Retryable: true, EffectsPossible: true},
	} {
		if CanAdvanceChain(b) {
			t.Fatalf("unsafe boundary accepted: %+v", b)
		}
	}
	base.EffectsPossible, base.BeforeDispatch = true, true
	if !CanAdvanceChain(base) {
		t.Fatal("pre-dispatch failure should advance")
	}
}
