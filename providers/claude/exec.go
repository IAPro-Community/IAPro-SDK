// Package claude implements Claude Code's structured CLI protocol through a
// trusted host launch plan. No credentials, permission policy or flags are
// chosen by this package.
package claude

import (
	"context"

	"github.com/IAPro-Community/IAPro-SDK/providers"
	parser "github.com/IAPro-Community/IAPro-SDK/providers/parsers/claude"
	"github.com/IAPro-Community/IAPro-SDK/providers/process"
)

// RunFreshExec requires a new, single-shot Claude Code stream-json session.
// The host must not launch with resume/continue or multi-turn input, because
// restored cumulative usage must not be attributed to a new run.
func RunFreshExec(ctx context.Context, launch process.Launch, prompt string, emit func(providers.Event) error) (providers.RunResult, error) {
	return process.Run(ctx, launch, prompt, parser.NewExecDecoder(emit))
}
