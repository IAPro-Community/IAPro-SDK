package providers

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBindings(t *testing.T) {
	b := NewBindings[int]()
	if err := b.Register(" Codex ", 1); err != nil {
		t.Fatal(err)
	}
	if err := b.Register("codex", 2); err == nil {
		t.Fatal("duplicate provider accepted")
	}
	if got, ok := b.Get("CODEX"); !ok || got != 1 {
		t.Fatal("normalized lookup failed")
	}
	got := DetectBindings(context.Background(), b.List(), time.Second, 2, func(_ context.Context, v int) int { return v + 1 })
	if got["codex"] != 2 {
		t.Fatal(got)
	}
}

type installStore struct {
	records []InstallationRecord
	fail    bool
}

func (s *installStore) Load() ([]InstallationRecord, error) { return s.records, nil }
func (s *installStore) Save(r []InstallationRecord) error {
	if s.fail {
		return errors.New("store failed")
	}
	s.records = r
	return nil
}

func TestInstallationDiscoveryDoesNotRegisterAccount(t *testing.T) {
	store := &installStore{}
	r, err := NewInstallationRegistry(store, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Apply(map[string]Detection{"codex": {Installed: true, BinaryPath: "/bin/codex", Version: "1"}})
	if err != nil || got[0].State != InstalledUnregistered {
		t.Fatalf("discovery: %+v %v", got, err)
	}
	store.fail = true
	if _, err := r.Apply(map[string]Detection{"codex": {}}); err == nil || !r.List()[0].Installed {
		t.Fatal("failed save changed published state")
	}
	store.fail = false
	got, err = r.Apply(map[string]Detection{"codex": {}})
	if err != nil || got[0].Installed || got[0].Path != "" {
		t.Fatal("stale executable metadata")
	}
}

func TestAccountManagerRejectsDuplicateAndInvalidKey(t *testing.T) {
	m := NewAccountManager[string]()
	if err := m.Register("codex", "host-port"); err != nil {
		t.Fatal(err)
	}
	if err := m.Register("codex", "other"); err == nil {
		t.Fatal("duplicate binding")
	}
	p, l, err := ParseAccountKey(AccountKey("codex", "default"))
	if err != nil || p != "codex" || l != "default" {
		t.Fatal("key roundtrip")
	}
	if _, _, err := ParseAccountKey("codex/a/b"); err == nil {
		t.Fatal("invalid key accepted")
	}
}
