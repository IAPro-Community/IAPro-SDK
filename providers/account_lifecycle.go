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

// ProfileRegistration carries a local name, not a remote identity or proof of
// authentication. OwnerID must come from the authorized host, never discovery.
type ProfileRegistration struct {
	OwnerID    string
	ProviderID string
	Label      string
}

// RegisterProfile performs shared pre-auth validation before committing through
// the host's create port. The commit must enforce uniqueness atomically and
// must not silently import credentials or start a login. Label rules remain a
// host compatibility policy (legacy profiles need not use portable labels).
func RegisterProfile[T any](ctx context.Context, req ProfileRegistration, providerAllowed func(string) error, validateLabel func(string) error, commit func(context.Context, ProfileRegistration) (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if strings.TrimSpace(req.OwnerID) == "" || req.ProviderID == "" || req.ProviderID != NormalizeID(req.ProviderID) || strings.TrimSpace(req.Label) == "" {
		return zero, errors.New("invalid profile registration")
	}
	if providerAllowed == nil || validateLabel == nil || commit == nil {
		return zero, errors.New("profile registration ports required")
	}
	if err := providerAllowed(req.ProviderID); err != nil {
		return zero, err
	}
	if err := validateLabel(req.Label); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return commit(ctx, req)
}

// AuthAction is an explicit operation, never triggered by inspection.
type AuthAction string

const (
	AuthLogin  AuthAction = "LOGIN"
	AuthLogout AuthAction = "LOGOUT"
)

// ExecuteAuth requires a host policy check and registered lifecycle port. It
// never interprets a successful command as verified account identity.
func ExecuteAuth(ctx context.Context, ref ProfileRef, action AuthAction, authorize func(context.Context, ProfileRef, AuthAction) error, lifecycle AccountLifecycle) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(ref.OwnerID) == "" || ref.ProviderID == "" || ref.ProviderID != NormalizeID(ref.ProviderID) || strings.TrimSpace(ref.ProfileID) == "" || authorize == nil || lifecycle == nil {
		return errors.New("authorized profile and lifecycle ports required")
	}
	if action != AuthLogin && action != AuthLogout {
		return errors.New("unknown auth action")
	}
	if err := authorize(ctx, ref, action); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if action == AuthLogin {
		return lifecycle.Login(ctx, ref)
	}
	return lifecycle.Logout(ctx, ref)
}
