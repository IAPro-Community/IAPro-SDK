package codex

import (
	"errors"
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

func TestExecDecoderClassifiesUnsupportedModel(t *testing.T) {
	for _, raw := range []string{
		`{"type":"error","message":"The 'gpt-5' model is not supported when using Codex with a ChatGPT account."}` + "\n",
		`{"type":"turn.failed","error":{"message":"The 'gpt-5' model is not supported when using Codex with a ChatGPT account."}}` + "\n",
	} {
		d := NewExecDecoder(nil)
		if _, err := d.Write([]byte(raw)); !errors.Is(err, ErrIncompleteTurn) {
			t.Fatalf("expected a failed turn, got %v", err)
		}
		result, err := d.Finish()
		if !errors.Is(err, ErrIncompleteTurn) {
			t.Fatalf("expected an incomplete turn, got %v", err)
		}
		if result.Failure.Kind != model.FailureModelUnsupported {
			t.Fatalf("expected unsupported model, got %+v", result.Failure)
		}
		if result.Failure.Message != "Requested model is not supported by this account" {
			t.Fatalf("failure message must be sanitized, got %q", result.Failure.Message)
		}
	}
}

func TestExecDecoderKeepsGenericFailureForUnknownText(t *testing.T) {
	d := NewExecDecoder(nil)
	if _, err := d.Write([]byte(`{"type":"turn.failed","error":{"message":"secret"}}` + "\n")); !errors.Is(err, ErrIncompleteTurn) {
		t.Fatalf("expected a failed turn, got %v", err)
	}
	result, _ := d.Finish()
	if result.Failure.Kind != model.FailureUnknown {
		t.Fatalf("expected unknown failure, got %+v", result.Failure)
	}
}
