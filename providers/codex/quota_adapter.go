// SPDX-License-Identifier: MIT

package codex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers"
	parser "github.com/IAPro-Community/IAPro-SDK/providers/parsers/codex"
)

// DedicatedQuotaConfig contains host-authorized ports. The host establishes
// exclusive access to a dedicated profile, secret ownership and safe launch.
// ReadAuth returns an owned buffer that the adapter wipes after use.
type DedicatedQuotaConfig struct {
	Home      string
	ReadAuth  func(context.Context) ([]byte, error)
	Lock      func(context.Context) (func(), error)
	Login     Launch
	AppServer Launch
	SameHome  func(string, string) bool
	Now       func() time.Time
}

// DedicatedQuota supplies the shared identity/authentication/probe lifecycle.
// A local auth document is attribution only; the RPC verifies remote identity.
type DedicatedQuota struct {
	cfg DedicatedQuotaConfig
}

var _ providers.AuthProvider = (*DedicatedQuota)(nil)
var _ providers.UsageProvider = (*DedicatedQuota)(nil)

func NewDedicatedQuota(cfg DedicatedQuotaConfig) (*DedicatedQuota, error) {
	if cfg.Home == "" || cfg.ReadAuth == nil || cfg.Lock == nil || cfg.Login == nil ||
		cfg.AppServer == nil || cfg.SameHome == nil || cfg.Now == nil {
		return nil, errors.New("dedicated Codex quota requires authorized host ports")
	}
	return &DedicatedQuota{cfg: cfg}, nil
}

func (q *DedicatedQuota) Identity(ctx context.Context, owner, profile string) (providers.Account, error) {
	if owner == "" || profile == "" {
		return providers.Account{}, providers.ErrIdentityMismatch
	}
	raw, err := q.cfg.ReadAuth(ctx)
	defer clear(raw)
	if err != nil || ctx.Err() != nil {
		return providers.Account{}, providers.ErrIdentityMismatch
	}
	id, err := ExternalAccountID(raw)
	if err != nil {
		return providers.Account{}, providers.ErrIdentityMismatch
	}
	generation := sha256.Sum256(raw)
	return providers.Account{OwnerID: owner, Scope: model.AccountScope{
		ProviderID: string(model.ProviderCodex), ProfileID: profile, AccountID: id,
		IdentityVersion: hex.EncodeToString(generation[:]), CredentialScope: "dedicated-codex-home",
	}}, nil
}

func (q *DedicatedQuota) verify(ctx context.Context, a providers.Account) error {
	if a.Validate() != nil {
		return providers.ErrIdentityMismatch
	}
	current, err := q.Identity(ctx, a.OwnerID, a.Scope.ProfileID)
	if err != nil || current != a {
		return providers.ErrIdentityMismatch
	}
	return ctx.Err()
}

func (q *DedicatedQuota) InspectAuth(ctx context.Context, a providers.Account) (providers.Authentication, error) {
	release, err := q.cfg.Lock(ctx)
	if err != nil || release == nil {
		return providers.Authentication{}, providers.ErrProbeFailed
	}
	defer release()
	if err := q.verify(ctx, a); err != nil {
		return providers.Authentication{}, err
	}
	ok, err := CheckLoginStatus(ctx, q.cfg.Login)
	if err != nil || !ok {
		return providers.Authentication{}, providers.ErrIdentityMismatch
	}
	if err := q.verify(ctx, a); err != nil {
		return providers.Authentication{}, err
	}
	return providers.Authentication{Scope: a.Scope, Authenticated: true, ObservedAt: q.cfg.Now()}, nil
}

func (q *DedicatedQuota) GetUsage(ctx context.Context, a providers.Account) (model.UsageSnapshot, error) {
	release, err := q.cfg.Lock(ctx)
	if err != nil || release == nil {
		return model.UsageSnapshot{}, providers.ErrProbeFailed
	}
	defer release()
	if err := q.verify(ctx, a); err != nil {
		return model.UsageSnapshot{}, err
	}
	rpc, err := StartStdioRPC(ctx, q.cfg.AppServer)
	if err != nil {
		return model.UsageSnapshot{}, err
	}
	snapshot, err := parser.Probe(ctx, rpc, a.Scope, a.Scope.AccountID, q.cfg.Home, q.cfg.SameHome, q.cfg.Now)
	if err != nil {
		return model.UsageSnapshot{}, err
	}
	if err := q.verify(ctx, a); err != nil {
		return model.UsageSnapshot{}, err
	}
	return snapshot, nil
}
