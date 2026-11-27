// Package process runs host-authorized structured CLI turns. The host chooses
// the verified binary, account environment, flags, permissions and workspace.
package process

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/IAPro-Community/IAPro-SDK/classifier"
	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers"
)

type Launch func(context.Context) (*exec.Cmd, error)

type Decoder interface {
	io.Writer
	Finish() (providers.RunResult, error)
}

const (
	// maxStderrBytes bounds how much of the provider's stderr is inspected.
	maxStderrBytes = 8 << 10
	// maxDetailBytes bounds the text handed to the classifier.
	maxDetailBytes = 16 << 10
)

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
	// stderr is kept off the pipe so a chatty CLI cannot block on a full buffer;
	// only its final bytes are inspected for classification, never echoed.
	stderr := &boundedTail{limit: maxStderrBytes}
	cmd.Stderr = stderr
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
		result = withClassifiedFailure(result, stderr.Tail())
		return result, ErrProtocol
	}
	if runErr != nil {
		result = withClassifiedFailure(result, stderr.Tail())
		return result, ErrProcess
	}
	return result, nil
}

// withClassifiedFailure fills a missing failure class and never rewrites raw
// provider text into the result. The turn diagnostic wins over process stderr
// because it describes the reason the provider itself reported.
func withClassifiedFailure(result providers.RunResult, stderr string) providers.RunResult {
	if result.Failure.Kind != "" && result.Failure.Kind != model.FailureNone &&
		result.Failure.Kind != model.FailureUnknown {
		return result
	}
	detail := stderr
	if result.Failure.Message != "" {
		detail = result.Failure.Message + "\n" + stderr
	}
	if len(detail) > maxDetailBytes {
		detail = detail[len(detail)-maxDetailBytes:]
	}
	classified := classifier.Classify(&providerExit{}, detail)
	if classified.Kind == model.FailureNone {
		classified = model.Failure{Kind: model.FailureUnknown, Message: "provider process failed"}
	}
	result.Failure = classified
	return result
}

// providerExit stands in for the failed process when only diagnostics exist, so
// the classifier never reports "no failure".
type providerExit struct{}

func (*providerExit) Error() string { return "provider process exited" }

// boundedTail keeps the most recent bytes written to it and drops the rest.
type boundedTail struct {
	mu    sync.Mutex
	buf   []byte
	limit int
}

func (b *boundedTail) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - b.limit; over > 0 {
		b.buf = append(b.buf[:0], b.buf[over:]...)
	}
	return len(p), nil
}

// Tail returns the retained stderr bytes as text.
func (b *boundedTail) Tail() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
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
