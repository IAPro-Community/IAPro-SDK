package codex

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers"
)

// RPC is a host-owned, isolated stdio app-server connection. The host must
// acquire the profile maintenance lock and bound/cancel transport I/O.
type RPC interface {
	Call(context.Context, string, any) (json.RawMessage, error)
	Notify(context.Context, string, any) error
	Close() error
}

// Probe performs the read-only handshake and quota request on an owned
// connection. No thread is created; the connection is always closed.
func Probe(ctx context.Context, rpc RPC, scope model.AccountScope, expectedAccount, expectedHome string, sameHome func(string, string) bool, now func() time.Time) (model.UsageSnapshot, error) {
	if rpc == nil {
		return model.UsageSnapshot{}, errors.New("codex quota transport required")
	}
	defer func() { _ = rpc.Close() }()
	if sameHome == nil || now == nil || expectedHome == "" || expectedAccount == "" || scope.ProviderID != "codex" || !scope.Verifiable() {
		return model.UsageSnapshot{}, errors.New("codex probe identity required")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	raw, err := rpc.Call(ctx, "initialize", map[string]any{"clientInfo": map[string]string{"name": "iapro_sdk", "version": "0.1.0"}})
	if err != nil {
		return model.UsageSnapshot{}, sanitizedProbeError(err, "codex initialize failed")
	}
	var init struct {
		CodexHome string `json:"codexHome"`
	}
	if len(raw) > maxEventBytes || json.Unmarshal(raw, &init) != nil || init.CodexHome == "" || !sameHome(init.CodexHome, expectedHome) {
		return model.UsageSnapshot{}, errors.New("codex app-server home mismatch")
	}
	if err := rpc.Notify(ctx, "initialized", nil); err != nil {
		return model.UsageSnapshot{}, errors.New("codex initialization notification failed")
	}
	raw, err = rpc.Call(ctx, "account/rateLimits/read", map[string]bool{"excludeResetCreditDetails": true})
	if err != nil {
		return model.UsageSnapshot{}, sanitizedProbeError(err, "codex quota read failed")
	}
	return ParseUsage(raw, scope, expectedAccount, now())
}

func sanitizedProbeError(err error, fallback string) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	var typed *providers.ProbeError
	if errors.As(err, &typed) {
		return &providers.ProbeError{Code: providers.ClassifyProbeError(err)}
	}
	return errors.New(fallback)
}
