// SPDX-License-Identifier: MIT

// Package codex contains executable protocol adapters with host-owned launch
// policy. It does not choose account credentials, workspace, or approvals.
package codex

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/providers"
	parser "github.com/IAPro-Community/IAPro-SDK/providers/parsers/codex"
)

// Launch constructs a fresh exec.CommandContext using the supplied context.
// The trusted host chooses a verified executable, environment, workspace and
// authorized flags. Never derive launch flags from a model message.
type Launch func(context.Context) (*exec.Cmd, error)

var ErrLaunch = errors.New("codex process launch failed")
var ErrProcess = errors.New("codex process failed")
var ErrProtocol = errors.New("codex event protocol failed")

// RunExec executes a host-authorized codex exec --json invocation. Prompt
// travels via stdin. No automatic retry, raw stderr or command logging.
func RunExec(ctx context.Context, launch Launch, prompt string, emit func(providers.Event) error) (providers.RunResult, error) {
	if err := ctx.Err(); err != nil {
		return providers.RunResult{}, err
	}
	if launch == nil {
		return providers.RunResult{}, ErrLaunch
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd, err := launch(runCtx)
	if err != nil || cmd == nil || cmd.Process != nil {
		return providers.RunResult{}, ErrLaunch
	}
	d := parser.NewExecDecoder(emit)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout = &cancelWriter{writer: d, cancel: cancel}
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 2 * time.Second
	runErr := cmd.Run()
	if cmd.Process == nil {
		return providers.RunResult{}, ErrLaunch
	}
	result, decodeErr := d.Finish()
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if decodeErr != nil {
		return result, ErrProtocol
	}
	if runErr != nil {
		return result, ErrProcess
	}
	return result, nil
}

type cancelWriter struct {
	writer io.Writer
	cancel context.CancelFunc
}

func (w *cancelWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if err != nil {
		w.cancel()
	}
	return n, err
}
