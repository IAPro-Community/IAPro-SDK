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

func TestReconciliationAloneDoesNotAuthorizeReplay(t *testing.T) {
	b := ChainBoundary{Authorized: true, Retryable: true, EffectsPossible: true, EffectsReconciled: true}
	if CanAdvanceChain(b) {
		t.Fatal("known committed effects are not safe to replay")
	}
	b.NoSideEffects = true
	if !CanAdvanceChain(b) {
		t.Fatal("verified absence of effects should permit retry")
	}
}
