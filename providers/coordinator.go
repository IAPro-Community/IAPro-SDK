// SPDX-License-Identifier: MIT
package providers

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

// ProbeCode classifies quota probe failures deterministically across CLI, TUI, and Web.
type ProbeCode string

const (
	ProbeOK            ProbeCode = ""
	ProbeTimeout       ProbeCode = "TIMEOUT"
	ProbeAuthExpired   ProbeCode = "AUTH_EXPIRED"
	ProbeProfileLocked ProbeCode = "PROFILE_LOCKED"
	ProbeRateLimited   ProbeCode = "RATE_LIMITED"
	ProbeParseFailed   ProbeCode = "PARSE_FAILED"
	ProbeBinaryMissing ProbeCode = "BINARY_MISSING"
	ProbeUnsupported   ProbeCode = "UNSUPPORTED"
	ProbeNetworkError  ProbeCode = "NETWORK_ERROR"
	ProbeUnknownError  ProbeCode = "PROBE_FAILED"
)

// ProbeError carries a typed ProbeCode alongside the underlying error.
type ProbeError struct {
	Code    ProbeCode
	Message string
	Err     error
}

func (e *ProbeError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Code)
}

func (e *ProbeError) Unwrap() error { return e.Err }

// ClassifyProbeError extracts a ProbeCode from an arbitrary probe error.
func ClassifyProbeError(err error) ProbeCode {
	if err == nil {
		return ProbeOK
	}
	var pe *ProbeError
	if errors.As(err, &pe) && pe.Code != "" {
		return pe.Code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ProbeTimeout
	}
	if errors.Is(err, ErrIdentityMismatch) {
		return ProbeAuthExpired
	}
	if errors.Is(err, ErrUnsupported) {
		return ProbeUnsupported
	}
	return ProbeUnknownError
}

// BatchOptions bounds parallel quota refreshes.
type BatchOptions struct {
	MaxConcurrency  int
	PerProbeTimeout time.Duration
	GlobalTimeout   time.Duration
}

// ProbeOutcome reports the result and diagnostic code for one account refresh.
type ProbeOutcome struct {
	Account  Account
	Snapshot model.UsageSnapshot
	Code     ProbeCode
	Err      error
	Duration time.Duration
}

// UsageCoordinator executes bounded parallel quota refreshes with singleflight
// coalescing and structured diagnostics.
type UsageCoordinator struct {
	cache *UsageCache
	probe func(context.Context, Account) (model.UsageSnapshot, error)
}

// NewUsageCoordinator wraps a UsageCache for parallel batch refreshes.
func NewUsageCoordinator(cache *UsageCache) (*UsageCoordinator, error) {
	if cache == nil {
		return nil, errors.New("usage cache is required")
	}
	return &UsageCoordinator{
		cache: cache,
		probe: cache.GetUsage,
	}, nil
}

// NewDirectCoordinator creates a coordinator over an arbitrary singleflight probe function.
func NewDirectCoordinator(probe func(context.Context, Account) (model.UsageSnapshot, error)) (*UsageCoordinator, error) {
	if probe == nil {
		return nil, errors.New("probe function is required")
	}
	return &UsageCoordinator{probe: probe}, nil
}

// RefreshBatch refreshes all distinct accounts concurrently under MaxConcurrency
// and timeout budgets, returning outcomes in input order.
func (c *UsageCoordinator) RefreshBatch(ctx context.Context, accounts []Account, opts BatchOptions) []ProbeOutcome {
	if len(accounts) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	maxConc := opts.MaxConcurrency
	if maxConc <= 0 {
		maxConc = 4
	}
	perProbe := opts.PerProbeTimeout
	if perProbe <= 0 {
		perProbe = 8 * time.Second
	}
	batchCtx := ctx
	var batchCancel context.CancelFunc
	if opts.GlobalTimeout > 0 {
		batchCtx, batchCancel = context.WithTimeout(ctx, opts.GlobalTimeout)
		defer batchCancel()
	}

	outcomes := make([]ProbeOutcome, len(accounts))
	sem := make(chan struct{}, maxConc)

	var inFlightMu sync.Mutex
	inFlight := make(map[string]*inFlightProbe)

	var wg sync.WaitGroup
	for i, acc := range accounts {
		wg.Add(1)
		go func(idx int, account Account) {
			defer wg.Done()
			key := account.OwnerID + "|" + account.Scope.ProviderID + "|" + account.Scope.ProfileID + "|" + account.Scope.AccountID
			inFlightMu.Lock()
			if shared, ok := inFlight[key]; ok {
				inFlightMu.Unlock()
				select {
				case <-batchCtx.Done():
					outcomes[idx] = ProbeOutcome{Account: account, Code: ClassifyProbeError(batchCtx.Err()), Err: batchCtx.Err()}
				case <-shared.done:
					outcomes[idx] = shared.outcome
					outcomes[idx].Account = account
				}
				return
			}
			shared := &inFlightProbe{done: make(chan struct{})}
			inFlight[key] = shared
			inFlightMu.Unlock()

			defer func() {
				close(shared.done)
			}()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-batchCtx.Done():
				res := ProbeOutcome{Account: account, Code: ClassifyProbeError(batchCtx.Err()), Err: batchCtx.Err()}
				shared.outcome = res
				outcomes[idx] = res
				return
			}

			start := time.Now()
			probeCtx, cancel := context.WithTimeout(batchCtx, perProbe)
			snap, err := c.probe(probeCtx, account)
			cancel()

			code := ClassifyProbeError(err)
			if err == nil && snap.DiagnosticCode != "" {
				code = ProbeCode(snap.DiagnosticCode)
			}
			if err != nil && snap.DiagnosticCode == "" {
				snap.DiagnosticCode = string(code)
				if snap.Error == "" {
					snap.Error = err.Error()
				}
			}
			res := ProbeOutcome{
				Account:  account,
				Snapshot: snap,
				Code:     code,
				Err:      err,
				Duration: time.Since(start),
			}
			shared.outcome = res
			outcomes[idx] = res
		}(i, acc)
	}
	wg.Wait()
	return outcomes
}

type inFlightProbe struct {
	done    chan struct{}
	outcome ProbeOutcome
}
