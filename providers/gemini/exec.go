// Package gemini implements the Gemini CLI structured headless protocol.
// It does not decide authentication, account selection, sandbox or flags.
package gemini

import (
	"context"

	"github.com/IAPro-Community/IAPro-SDK/providers"
	parser "github.com/IAPro-Community/IAPro-SDK/providers/parsers/gemini"
	"github.com/IAPro-Community/IAPro-SDK/providers/process"
)

// RunExec executes a host-authorized Gemini stream-json launch using stdin.
func RunExec(ctx context.Context, launch process.Launch, prompt string, emit func(providers.Event) error) (providers.RunResult, error) {
	return process.Run(ctx, launch, prompt, parser.NewExecDecoder(emit))
}
