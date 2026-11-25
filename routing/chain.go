package routing

// ChainBoundary is evidence supplied by the host after an attempt. It is
// deliberately independent of quota selection: explicitly ordered accounts
// remain the host's policy; no account is automatically discovered here.
type ChainBoundary struct {
	Authorized        bool
	Retryable         bool
	Canceled          bool
	BudgetOrLedger    bool
	OutputEmitted     bool
	BeforeDispatch    bool
	EffectsPossible   bool
	EffectsReconciled bool
	NoSideEffects     bool
}

// CanAdvanceChain refuses ambiguous retries if a tool or native CLI could
// have produced side effects. An absence of streamed text is not proof of
// absence of effects. The host remains responsible for identity and budget.
func CanAdvanceChain(b ChainBoundary) bool {
	return b.Authorized && b.Retryable && !b.Canceled && !b.BudgetOrLedger &&
		!b.OutputEmitted && (b.BeforeDispatch || !b.EffectsPossible || (b.EffectsReconciled && b.NoSideEffects))
}
