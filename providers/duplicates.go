package providers

import (
	"sort"

	"github.com/IAPro-Community/IAPro-SDK/lease"
)

// DuplicateHint is local metadata only. It may inform a user but cannot
// authorize a merge, deletion, credential transfer or routing decision.
type DuplicateHint struct {
	OwnerID      string
	ProviderID   string
	ProfileID    string
	Identity     string
	AccountLabel string
	Disabled     bool
}

type DuplicateHintGroup struct {
	OwnerID      string
	ProviderID   string
	Identity     string
	AccountLabel string
	Profiles     []string
}

func GroupDuplicateHints(hints []DuplicateHint) []DuplicateHintGroup {
	type key struct{ owner, provider, identity string }
	groups := make(map[key]map[string]DuplicateHint)
	for _, h := range hints {
		if h.Disabled || h.OwnerID == "" || h.ProviderID == "" || h.Identity == "" || h.ProfileID == "" {
			continue
		}
		k := key{h.OwnerID, NormalizeID(h.ProviderID), h.Identity}
		if groups[k] == nil {
			groups[k] = make(map[string]DuplicateHint)
		}
		groups[k][h.ProfileID] = h
	}
	var out []DuplicateHintGroup
	for k, items := range groups {
		if len(items) < 2 {
			continue
		}
		g := DuplicateHintGroup{OwnerID: k.owner, ProviderID: k.provider, Identity: k.identity}
		for profile := range items {
			g.Profiles = append(g.Profiles, profile)
		}
		sort.Strings(g.Profiles)
		for _, profile := range g.Profiles {
			if label := items[profile].AccountLabel; label != "" {
				g.AccountLabel = label
				break
			}
		}
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.OwnerID != b.OwnerID {
			return a.OwnerID < b.OwnerID
		}
		if a.ProviderID != b.ProviderID {
			return a.ProviderID < b.ProviderID
		}
		return a.Identity < b.Identity
	})
	return out
}

// VerifiedAccountGroups groups independently verified accounts by physical
// identity, ignoring aliases and credential generation. Caller must establish
// verification; nonempty metadata alone is not proof.
func VerifiedAccountGroups(accounts []Account) map[lease.Key][]Account {
	out := make(map[lease.Key][]Account)
	for _, a := range accounts {
		if a.Validate() != nil {
			continue
		}
		key := lease.PhysicalKey(a.OwnerID, a.Scope)
		out[key] = append(out[key], a)
	}
	return out
}
