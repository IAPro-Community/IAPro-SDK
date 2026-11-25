package codex

import (
	"errors"
	"strings"
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/providers"
)

func TestExecDecoderStreamingAndUsage(t *testing.T) {
	var text []string
	d := NewExecDecoder(func(e providers.Event) error {
		text = append(text, e.Text)
		return nil
	})
	for _, line := range []string{
		`{"type":"thread.started","thread_id":"test"}` + "\n",
		`{"type":"item.completed","item":{"id":"1","type":"agent_message","text":"hello"}}` + "\n",
		`{"type":"item.completed","item":{"id":"1","type":"agent_message","text":"hello"}}` + "\n",
		`{"type":"turn.completed","usage":{"input_tokens":7,"output_tokens":3}}`,
	} {
		for i := range line {
			if _, err := d.Write([]byte(line[i : i+1])); err != nil {
				t.Fatal(err)
			}
		}
	}
	r, err := d.Finish()
	if err != nil || len(text) != 1 || text[0] != "hello" || r.SessionID != "test" || r.InputTokens == nil || *r.InputTokens != 7 || *r.OutputTokens != 3 {
		t.Fatalf("stream or usage mismatch: %v %+v %v", text, r, err)
	}
}

func TestExecDecoderFailsClosed(t *testing.T) {
	for _, raw := range []string{
		`{"type":"item.completed","item":{"id":"1","type":"agent_message","text":"partial"}}`,
		`{"type":"turn.failed","error":{"message":"secret"}}`,
		"not json",
		strings.Repeat("x", maxEventBytes+1),
	} {
		d := NewExecDecoder(nil)
		_, err := d.Write([]byte(raw))
		if err == nil {
			_, err = d.Finish()
		}
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("expected sanitized failure, got %v", err)
		}
	}
	d := NewExecDecoder(nil)
	if _, err := d.Finish(); !errors.Is(err, ErrIncompleteTurn) {
		t.Fatal(err)
	}
}
