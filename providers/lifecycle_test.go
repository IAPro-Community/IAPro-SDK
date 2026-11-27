package providers

import (
	"context"
	"errors"
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

func TestRegisterProfilePolicyBeforeCommit(t *testing.T) {
	calls := 0
	req := ProfileRegistration{OwnerID: "owner", ProviderID: "codex", Label: "test"}
	allow := func(string) error { return nil }
	deny := func(string) error { return errors.New("denied") }
	commit := func(context.Context, ProfileRegistration) (string, error) { calls++; return "created", nil }
	if _, err := RegisterProfile(context.Background(), req, deny, allow, commit); err == nil || calls != 0 {
		t.Fatal("unauthorized mutation")
	}
	if _, err := RegisterProfile(context.Background(), req, allow, deny, commit); err == nil || calls != 0 {
		t.Fatal("invalid label mutated state")
	}
	got, err := RegisterProfile(context.Background(), req, allow, allow, commit)
	if err != nil || got != "created" || calls != 1 {
		t.Fatal(got, err, calls)
	}
}

func TestDuplicateHintsAreOwnerScopedAndNonDestructive(t *testing.T) {
	hints := []DuplicateHint{
		{OwnerID: "one", ProviderID: "codex", ProfileID: "a", Identity: "id"},
		{OwnerID: "one", ProviderID: "codex", ProfileID: "b", Identity: "id"},
		{OwnerID: "two", ProviderID: "codex", ProfileID: "c", Identity: "id"},
		{OwnerID: "one", ProviderID: "codex", ProfileID: "a", Identity: "id"},
		{OwnerID: "one", ProviderID: "codex", ProfileID: "d", Identity: "id", Disabled: true},
	}
	got := GroupDuplicateHints(hints)
	if len(got) != 1 || len(got[0].Profiles) != 2 || got[0].Profiles[0] != "a" || got[0].Profiles[1] != "b" {
		t.Fatalf("unexpected hints: %+v", got)
	}
}

func TestVerifiedGroupsIgnoreAliasAndGenerationNotOwner(t *testing.T) {
	a := Account{OwnerID: "owner", Scope: model.AccountScope{ProviderID: "codex", ProfileID: "a", AccountID: "id", IdentityVersion: "1"}}
	b := a
	b.Scope.ProfileID = "b"
	b.Scope.IdentityVersion = "2"
	c := b
	c.OwnerID = "other"
	got := VerifiedAccountGroups([]Account{a, b, c, {}})
	if len(got) != 2 {
		t.Fatal(got)
	}
}
