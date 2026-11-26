// Package process runs host-authorized structured CLI turns. The host chooses
// the verified binary, account environment, flags, permissions and workspace.
package process

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/providers"
)

type Launch func(context.Context) (*exec.Cmd, error)

type Decoder interface {
	io.Writer
	Finish() (providers.RunResult, error)
}

var (
	ErrLaunch   = errors.New("provider process launch failed")
	ErrProcess  = errors.New("provider process failed")
	ErrProtocol = errors.New("provider event protocol failed")
	// ErrNotDispatched certifies that no model process was started.
	ErrNotDispatched = errors.New("provider process not dispatched")
)

// Run never retries or exposes raw diagnostics. The decoder must require a
// successful terminal event; successful process exit is independently required.
func Run(ctx context.Context, launch Launch, prompt string, decoder Decoder) (providers.RunResult, error) {
	if err := ctx.Err(); err != nil {
		return providers.RunResult{}, errors.Join(ErrNotDispatched, err)
	}
	if launch == nil || decoder == nil {
		return providers.RunResult{}, errors.Join(ErrNotDispatched, ErrLaunch)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd, err := launch(runCtx)
	if err != nil || cmd == nil || cmd.Process != nil {
		return providers.RunResult{}, errors.Join(ErrNotDispatched, ErrLaunch)
	}
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout = &cancelWriter{writer: decoder, cancel: cancel}
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 2 * time.Second
	runErr := cmd.Run()
	if cmd.Process == nil {
		if ctx.Err() != nil {
			return providers.RunResult{}, errors.Join(ErrNotDispatched, ctx.Err())
		}
		return providers.RunResult{}, errors.Join(ErrNotDispatched, ErrLaunch)
	}
	result, decodeErr := decoder.Finish()
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
