package routing

import (
	"errors"
	"strings"
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
	// Paid fallback needs a verified host budget reservation before selection.
	Reservation *BudgetReservation
	ExpiresAt   time.Time

	// QuotaScope is the identity bound to the observed quota, not the current login.
	QuotaScope  model.AccountScope
	Model       string
	QuotaGroup  string
	GroupMapped bool
}

// SwitchRequest describes a failed or unavailable source and the policy
// established by the host. No execution or credential transfer occurs here.
type BudgetReservation struct {
	ID          string
	OwnerID     string
	RunID       string
	Scope       model.AccountScope
	Model       string
	MaxMicroUSD int64
	ExpiresAt   time.Time
}

// SwitchRequest describes a failed or unavailable source and the policy.
type SwitchRequest struct {
	OwnerID            string
	RunID              string
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
		if c.OwnerID != req.OwnerID || !c.Scope.Verifiable() || !c.Authorized || !c.TermsPermit || !c.ModelCompatible || !c.GroupMapped || strings.TrimSpace(c.Model) == "" {
			continue
		}
		if c.Scope.ProviderID == req.Source.ProviderID && (c.Scope.AccountID == req.Source.AccountID || c.Scope.ProfileID == req.Source.ProfileID) {
			continue
		}
		if !req.AllowOtherProvider && c.Scope.ProviderID != req.Source.ProviderID {
			continue
		}
		if c.Paid && !paidAuthorized(req, c) {
			continue
		}
		// An automatic switch requires measured, unexpired evidence; manual
		// selection of unsupported or unknown quotas is a different operation.
		v := c.Eligibility.View
		if v.Status != string(model.UsageLive) && v.Status != string(model.UsageCached) {
			continue
		}
		if c.ExpiresAt.IsZero() || !req.Now.Before(c.ExpiresAt) || v.FetchedAt.IsZero() || v.FetchedAt.After(req.Now) || !c.ExpiresAt.After(v.FetchedAt) {
			continue
		}
		if v.Provider != c.Scope.ProviderID || v.Profile != c.Scope.ProfileID || c.QuotaScope != c.Scope || v.Scope != c.Scope ||
			v.ExpiresAt == nil || !req.Now.Before(*v.ExpiresAt) || c.ExpiresAt.After(*v.ExpiresAt) {
			continue
		}
		// Require the actual selected pool and provider-wide constraints.
		if !modelPoolAvailable(v, c.QuotaGroup, req.Now) {
			continue
		}
		in := c.Eligibility
		in.ExplicitSelection = false
		in.Now = req.Now
		in.View.ComputeAvailabilityAt(req.Now)
		if quota.EvaluateEligibility(in).Eligible {
			return c, nil
		}
	}
	return Account{}, ErrNoEligibleAccount
}

// modelPoolAvailable requires a unique selected group and every required
// window, including global constraints when a named group is selected.
func modelPoolAvailable(v quota.QuotaView, key string, now time.Time) bool {
	found := false
	globalFound := false
	for _, group := range v.ModelGroups {
		if group.Key != key && group.Key != "" {
			continue
		}
		if group.Key == key {
			if found {
				return false
			}
			found = true
		} else {
			if globalFound {
				return false
			}
			globalFound = true
		}
		if len(group.Windows) == 0 {
			return false
		}
		for _, w := range group.Windows {
			if w.Kind == "unknown" || (w.Status != string(model.UsageLive) && w.Status != string(model.UsageCached)) ||
				!(w.Remaining > 0 && w.Remaining <= 100) || (w.ResetTime != nil && !now.Before(*w.ResetTime)) {
				return false
			}
		}
	}
	return found
}

// paidAuthorized verifies the binding of a reservation issued atomically by
// the host ledger. The ledger must revalidate/consume the ID before dispatch.
func paidAuthorized(req SwitchRequest, c Account) bool {
	r := c.Reservation
	return req.AllowPaid && req.RunID != "" && r != nil && r.ID != "" && r.OwnerID == req.OwnerID &&
		r.RunID == req.RunID && r.Scope == c.Scope && r.Model == c.Model && r.MaxMicroUSD > 0 &&
		req.Now.Before(r.ExpiresAt)
}
