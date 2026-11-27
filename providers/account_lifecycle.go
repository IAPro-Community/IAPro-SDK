package providers

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

// Lifecycle ports receive only host-authorized operations. In particular,
// deletion and authentication must never be triggered by passive discovery.
// ProfileRef is a host-authorized reference that may be signed out. It is
// deliberately not a verified Account and cannot be used for routing.
type ProfileRef struct {
	OwnerID    string
	ProviderID string
	ProfileID  string
}

type AccountLifecycle interface {
	Login(context.Context, ProfileRef) error
	Logout(context.Context, ProfileRef) error
}

var portableLabel = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

func ValidPortableLabel(label string) bool { return portableLabel.MatchString(label) }

func AccountKey(provider, label string) string { return provider + "/" + label }

func ParseAccountKey(key string) (provider, label string, err error) {
	provider, label, ok := strings.Cut(key, "/")
	if !ok || provider == "" || label == "" || strings.Contains(label, "/") {
		return "", "", errors.New("invalid provider account key")
	}
	return provider, label, nil
}

// AccountManager resolves only a provider explicitly registered by the host.
// Host account persistence, credentials, policy and workspace remain external.
type AccountManager[T any] struct {
	bindings *Bindings[T]
}

func NewAccountManager[T any]() *AccountManager[T] {
	return &AccountManager[T]{bindings: NewBindings[T]()}
}

func (m *AccountManager[T]) Register(provider string, port T) error {
	return m.bindings.Register(provider, port)
}

func (m *AccountManager[T]) Lookup(provider string) (T, bool) {
	return m.bindings.Get(provider)
}

func (m *AccountManager[T]) List() []Binding[T] { return m.bindings.List() }
