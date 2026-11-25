// SPDX-License-Identifier: MIT

package codex

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/providers"
)

func TestDedicatedQuotaRequiresPortsAndBindsCredentialGeneration(t *testing.T) {
	if _, err := NewDedicatedQuota(DedicatedQuotaConfig{}); err == nil {
		t.Fatal("missing authorized ports accepted")
	}
	token := "fixture-one"
	var owned []byte
	read := func(context.Context) ([]byte, error) {
		owned = []byte(`{"tokens":{"account_id":"physical","access_token":"` + token + `"}}`)
		return owned, nil
	}
	launch := func(context.Context) (*exec.Cmd, error) { return nil, errors.New("fixture not launched") }
	q, err := NewDedicatedQuota(DedicatedQuotaConfig{
		Home: "dedicated", ReadAuth: read, Lock: func(context.Context) (func(), error) { return func() {}, nil },
		Login: launch, AppServer: launch, SameHome: func(a, b string) bool { return a == b }, Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	a, err := q.Identity(context.Background(), "owner", "profile")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range owned {
		if value != 0 {
			t.Fatal("auth buffer was not wiped")
		}
	}
	token = "fixture-two"
	b, err := q.Identity(context.Background(), "owner", "profile")
	if err != nil || a.Scope.AccountID != b.Scope.AccountID || a.Scope.IdentityVersion == b.Scope.IdentityVersion {
		t.Fatal("credential rotation did not change evidence generation")
	}
	if err := q.verify(context.Background(), a); !errors.Is(err, providers.ErrIdentityMismatch) {
		t.Fatal("old identity accepted")
	}
}
