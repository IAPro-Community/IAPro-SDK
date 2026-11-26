package process

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/model"
	"github.com/IAPro-Community/IAPro-SDK/providers"
)

// fakeDecoder records writes and returns a fixed result.
type fakeDecoder struct {
	out    strings.Builder
	result providers.RunResult
	err    error
}

func (d *fakeDecoder) Write(p []byte) (int, error) { return d.out.Write(p) }
func (d *fakeDecoder) Finish() (providers.RunResult, error) {
	return d.result, d.err
}

func shellCommand(ctx context.Context, script string) (*exec.Cmd, error) {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/c", script), nil
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", script), nil
}

func TestRunClassifiesStderrOfFailedProcess(t *testing.T) {
	decoder := &fakeDecoder{}
	_, err := Run(context.Background(), func(ctx context.Context) (*exec.Cmd, error) {
		return shellCommand(ctx, "echo 'the requested model is not supported' >&2; exit 1")
	}, "prompt", decoder)
	if !errors.Is(err, ErrProcess) {
		t.Fatalf("expected ErrProcess, got %v", err)
	}
}

func TestRunKeepsDecodedFailureWhenItIsSpecific(t *testing.T) {
	decoder := &fakeDecoder{
		result: providers.RunResult{Failure: model.Failure{Kind: model.FailureAuth, Message: "Authentication required"}},
		err:    errors.New("incomplete"),
	}
	result, err := Run(context.Background(), func(ctx context.Context) (*exec.Cmd, error) {
		return shellCommand(ctx, "echo 'rate limit' >&2; exit 0")
	}, "prompt", decoder)
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected ErrProtocol, got %v", err)
	}
	if result.Failure.Kind != model.FailureAuth {
		t.Fatalf("decoded failure must win, got %+v", result.Failure)
	}
}

func TestRunClassifiesStderrWhenDecoderHasNoFailure(t *testing.T) {
	decoder := &fakeDecoder{result: providers.RunResult{Failure: model.Failure{Kind: model.FailureUnknown}}}
	result, err := Run(context.Background(), func(ctx context.Context) (*exec.Cmd, error) {
		return shellCommand(ctx, "echo \"The 'gpt-5' model is not supported when using Codex with a ChatGPT account.\" >&2; exit 1")
	}, "prompt", decoder)
	if !errors.Is(err, ErrProcess) {
		t.Fatalf("expected ErrProcess, got %v", err)
	}
	if result.Failure.Kind != model.FailureModelUnsupported {
		t.Fatalf("expected unsupported model, got %+v", result.Failure)
	}
	if strings.Contains(result.Failure.Message, "gpt-5") {
		t.Fatalf("raw diagnostics must not leak: %q", result.Failure.Message)
	}
}

func TestBoundedTailKeepsTheEnd(t *testing.T) {
	tail := &boundedTail{limit: 8}
	if _, err := tail.Write([]byte("0123456789abcdef")); err != nil {
		t.Fatal(err)
	}
	if got := tail.Tail(); got != "89abcdef" {
		t.Fatalf("unexpected tail %q", got)
	}
}
