// SPDX-License-Identifier: MIT

package lease

import (
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

func TestPhysicalKeyIgnoresAliasesAndCredentialRotation(t *testing.T) {
	a := model.AccountScope{ProviderID: "codex", ProfileID: "a", AccountID: "physical", IdentityVersion: "one"}
	b := a
	b.ProfileID, b.IdentityVersion = "b", "two"
	k := PhysicalKey("owner", a)
	if !k.valid() || k != PhysicalKey("owner", b) || k.AccountID == a.AccountID {
		t.Fatal("physical account identity not preserved")
	}
	if k == PhysicalKey("different-owner", a) || PhysicalKey("owner", model.AccountScope{}).valid() {
		t.Fatal("invalid or foreign scope accepted")
	}
}
