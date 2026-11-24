package routing

import (
	"errors"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/quota"
)

var ErrNoEligibleAccount = errors.New("no authorized eligible fallback account")
var ErrUnsafeSwitch = errors.New("account switch requires authorization and a safe execution boundary")

// Account is supplied by the trusted host, never by an agent prompt.
// Candidates arrive in the host's canonical ranking order.
type Account struct {
	OwnerID         string
	Scope           model.AccountScope
	Eligibility     quota.EligibilityInput
	Authorized      bool
	TermsPermit     bool
	ModelCompatible bool
	Paid            bool
	ExpiresAt       time.Time
}

// SwitchRequest describes a failed or unavailable source and the policy
// established by the host. No execution or credential transfer occurs here.
type SwitchRequest struct {
	OwnerID            string
	Source             model.AccountScope
	AllowSwitch        bool
	AllowOtherProvider bool
	AllowPaid          bool
	BeforeDispatch     bool
	Reconciled         bool
	NoOutput           bool
	NoSideEffects      bool
	Now                time.Time
}

// SelectFallback applies common eligibility and safety gates to the host's
// ordered candidates. A positive decision is advisory; the host must reserve
// the account atomically, revalidate identity and policy, then dispatch.
func SelectFallback(req SwitchRequest, candidates []Account) (Account, error) {
	if !req.AllowSwitch || req.OwnerID == "" || !req.Source.Verifiable() || req.Now.IsZero() {
		return Account{}, ErrUnsafeSwitch
	}
	if !req.BeforeDispatch && !(req.Reconciled && req.NoOutput && req.NoSideEffects) {
		return Account{}, ErrUnsafeSwitch
	}
	for _, c := range candidates {
		if c.OwnerID != req.OwnerID || !c.Scope.Verifiable() || !c.Authorized || !c.TermsPermit || !c.ModelCompatible {
			continue
		}
		if c.Scope.ProviderID == req.Source.ProviderID && (c.Scope.AccountID == req.Source.AccountID || c.Scope.ProfileID == req.Source.ProfileID) {
			continue
		}
		if !req.AllowOtherProvider && c.Scope.ProviderID != req.Source.ProviderID {
			continue
		}
		if c.Paid && !req.AllowPaid {
			continue
		}
		// An automatic switch requires measured, unexpired evidence; manual
		// selection of unsupported or unknown quotas is a different operation.
		v := c.Eligibility.View
		if v.Status != string(model.UsageLive) && v.Status != string(model.UsageCached) {
			continue
		}
		if c.ExpiresAt.IsZero() || !req.Now.Before(c.ExpiresAt) || v.FetchedAt.IsZero() || v.FetchedAt.After(req.Now) {
			continue
		}
		if v.Provider != c.Scope.ProviderID || v.Profile != c.Scope.ProfileID {
			continue
		}
		// At least one independent pool must have a current measured window.
		// The host's ModelCompatible must also verify the selected model group.
		known := false
		for _, group := range v.ModelGroups {
			current := false
			blocked := false
			for _, w := range group.Windows {
				if w.Kind == "unknown" || (w.ResetTime != nil && !req.Now.Before(*w.ResetTime)) {
					continue
				}
				if w.Remaining > 0 && w.Remaining <= 100 {
					current = true
				} else {
					blocked = true
				}
			}
			if current && !blocked {
				known = true
				break
			}
		}
		if !known {
			continue
		}
		in := c.Eligibility
		in.ExplicitSelection = false
		if quota.EvaluateEligibility(in).Eligible {
			return c, nil
		}
	}
	return Account{}, ErrNoEligibleAccount
}
