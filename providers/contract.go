// SPDX-License-Identifier: MIT
// Provider-neutral host ports adapted from the Nexus provider contract.

package providers

import (
	"context"
	"errors"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

var ErrUnsupported = errors.New("provider capability unsupported")

// Account binds host authorization to a verified login and isolated runtime.
// The host owns secrets; credentials must never be stored in these DTOs.
type Account struct {
	OwnerID string
	Scope   model.AccountScope
}

func (a Account) Validate() error {
	if a.OwnerID == "" || !a.Scope.Verifiable() {
		return errors.New("owner and verifiable account scope required")
	}
	return nil
}

type Detection struct {
	Installed bool
	Version   string
}

type Authentication struct {
	Scope          model.AccountScope
	Authenticated bool
	ObservedAt     time.Time
}

// Adapter is the shared contract, not an execution authority. The host
// authorizes all calls and owns approvals, workspace and persistence.
type Adapter interface {
	ID() model.ProviderID
	Detect(context.Context) (Detection, error)
	Capabilities() model.Capabilities
}

type AuthProvider interface {
	InspectAuth(context.Context, Account) (Authentication, error)
}

type UsageProvider interface {
	GetUsage(context.Context, Account) (model.UsageSnapshot, error)
}

type RunRequest struct {
	Account   Account
	RunID     string
	Model     string
	Prompt    string
	SessionID string
}

type EventKind string

const (
	EventText       EventKind = "TEXT"
	EventSession    EventKind = "SESSION"
	EventDiagnostic EventKind = "DIAGNOSTIC"
)

type Event struct {
	Kind      EventKind
	Text      string
	SessionID string
}

type RunResult struct {
	SessionID    string
	InputTokens  *int64
	OutputTokens *int64
	Failure      model.Failure
}

// ExecutionProvider streams typed events. No automatic retry is implied.
// Host must prove the authorization, budget and sandbox boundary.
type ExecutionProvider interface {
	Run(context.Context, RunRequest, func(Event) error) (RunResult, error)
	MayHaveSideEffects() bool
}

// SessionProvider can verify continuity, not infer it from command success.
type SessionProvider interface {
	CanResume(context.Context, Account, string) (bool, error)
}
