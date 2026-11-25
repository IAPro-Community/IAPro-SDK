// SPDX-License-Identifier: MIT

// Package codex contains executable protocol adapters with host-owned launch
// policy. It does not choose account credentials, workspace, or approvals.
package codex

import (
	"context"

	"github.com/IAPro-Community/IAPro-SDK/providers"
	parser "github.com/IAPro-Community/IAPro-SDK/providers/parsers/codex"
	"github.com/IAPro-Community/IAPro-SDK/providers/process"
)

type Launch = process.Launch

var (
	ErrLaunch   = process.ErrLaunch
	ErrProcess  = process.ErrProcess
	ErrProtocol = process.ErrProtocol
)

// RunExec executes a host-authorized codex exec --json invocation. Prompt
// travels via stdin. No automatic retry, raw stderr or command logging.
func RunExec(ctx context.Context, launch Launch, prompt string, emit func(providers.Event) error) (providers.RunResult, error) {
	return process.Run(ctx, launch, prompt, parser.NewExecDecoder(emit))
}
