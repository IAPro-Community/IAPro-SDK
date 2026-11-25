// SPDX-License-Identifier: MIT

package lease

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

// PhysicalKey excludes aliases and credential generations from a lease key.
// The caller must use a canonical owner shared by participating hosts.
// Hashing avoids storing the raw provider account ID; it is not anonymization.
func PhysicalKey(owner string, scope model.AccountScope) Key {
	if owner == "" || !scope.Verifiable() {
		return Key{}
	}
	digest := sha256.Sum256([]byte(scope.AccountID))
	return Key{OwnerID: owner, Provider: scope.ProviderID, AccountID: hex.EncodeToString(digest[:])}
}
